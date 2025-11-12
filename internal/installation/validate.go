// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package installation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/furyctl/internal/distribution"
)

// Static error definitions for linting compliance.
var (
	ErrFlagsMustBeObject            = errors.New("flags section must be an object")
	ErrUnsupportedFlagsCommand      = errors.New("unsupported flags command")
	ErrFlagsValidationFailed        = errors.New("flags validation failed")
	ErrExpandedConfigurationNotAMap = errors.New("expanded configuration is not a map[string]any")
	ErrReadingSpec                  = errors.New("error reading spec from cluster's furyctl")
	ErrValidatingInstaller          = errors.New("error during installer validation")
	ErrValidating                   = errors.New("some tests were not successfull, check the logs")
)

func Validate(furyctlContent string, kind string, repoPath string) error {

	switch kind {

	case distribution.EKSClusterKind:
		validator := EKSInstallationValidator{
			furyctlContent: furyctlContent,
			repoPath:       repoPath,
		}
		return validator.Validate()

	case distribution.OnPremisesKind:
		validator := OnPremisesInstallationValidator{
			furyctlContent: furyctlContent,
			repoPath:       repoPath,
		}
		return validator.Validate()

	case distribution.KFDDistributionKind:
		validator := KFDInstallationValidator{
			furyctlContent: furyctlContent,
			repoPath:       repoPath,
		}
		return validator.Validate()
	}

	return nil
}

// TestEvent mirrors the fields produced by `go test -json`.
type TestEvent struct {
	Time    time.Time `json:"Time"`              // RFC3339 timestamps
	Action  string    `json:"Action"`            // run|pause|cont|pass|fail|skip|output
	Package string    `json:"Package,omitempty"` // import path
	Test    string    `json:"Test,omitempty"`    // test or subtest name
	Elapsed float64   `json:"Elapsed,omitempty"` // seconds on pass/fail of a test/pkg
	Output  string    `json:"Output,omitempty"`  // stdout/stderr chunk
}

// Result is a simple aggregation of outcomes.
type Result struct {
	Packages map[string]PkgSummary
	NumPass  int
	NumFail  int
	NumSkip  int
	Duration time.Duration
}

type PkgSummary struct {
	Package  string
	Passed   bool
	Failed   bool
	Duration time.Duration
}

func RunTests(component, dir string) error {

	logrus.Info(fmt.Sprintf("Running %s tests", component))
	args := []string{"test", "-json", "-count", "1", "-tags", "integration"}
	cmd := exec.CommandContext(context.TODO(), "go", args...)
	cmd.Dir = dir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start go test: %w", err)
	}

	dec := json.NewDecoder(stdout)
	events := make([]TestEvent, 0, 256)
	summary := Result{Packages: make(map[string]PkgSummary)}

	streamStart := time.Now()
	for {
		var ev TestEvent
		if err := dec.Decode(&ev); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			// Skip malformed lines but continue parsing.
			continue
		}
		events = append(events, ev)

		// Aggregate basic stats.
		switch ev.Action {
		case "pass":
			if ev.Test != "" {
				summary.NumPass++
			} else if ev.Package != "" {
				ps := summary.Packages[ev.Package]
				ps.Package = ev.Package
				ps.Passed = true
				ps.Duration = time.Duration(ev.Elapsed * float64(time.Second))
				summary.Packages[ev.Package] = ps
			}
		case "fail":
			logrus.Error(ev.Test)
			if ev.Test != "" {
				summary.NumFail++
			} else if ev.Package != "" {
				ps := summary.Packages[ev.Package]
				ps.Package = ev.Package
				ps.Failed = true
				ps.Duration = time.Duration(ev.Elapsed * float64(time.Second))
				summary.Packages[ev.Package] = ps
			}
		case "skip":
			if ev.Test != "" {
				summary.NumSkip++
			}
		}
	}
	summary.Duration = time.Since(streamStart)

	waitErr := cmd.Wait()
	// If go test failed to run at all, surface stderr to help debugging.
	if waitErr != nil && len(events) == 0 {
		stderr := stderrBuf.String()
		if !strings.HasPrefix(stderr, "no Go files in") {
			return fmt.Errorf("go test failed: %v\n%s", waitErr, stderr)
		}
	}
	logrus.Info("Tests for %s: ok", component)
	return nil
}
