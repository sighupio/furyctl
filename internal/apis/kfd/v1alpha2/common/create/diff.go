// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package create

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/tool/kapp"
	"github.com/sighupio/furyctl/internal/tool/shell"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

const (
	// ReasonKappFormat is the reason of a section whose kapp output is missing or invalid.
	ReasonKappFormat = "kapp output did not match the expected format"

	distributionTitle = "Distribution"
)

// CaptureRunner runs a script and returns its combined output and its exit code.
type CaptureRunner interface {
	RunCapture(args ...string) (string, int, error)
}

// DiffDistribution compares the distribution phase by running its rendered scripts/diff.sh. The runner must run
// the script in the manifests folder of the phase.
func DiffDistribution(distroPath, phasePath string, r CaptureRunner) Section {
	if section, supported := checkDiffSupport(distributionTitle, distroPath, "distribution"); !supported {
		return section
	}

	manifestsPath := filepath.Join(phasePath, "manifests")

	out, code, err := runDiffScript(r, filepath.Join(phasePath, "scripts", "diff.sh"), manifestsPath)
	if err != nil {
		return Section{Title: distributionTitle, Status: StatusFailed, Reason: err.Error()}
	}

	if code != 0 {
		return failedScript(distributionTitle, code, out, manifestsPath)
	}

	return kappSection(distributionTitle, filepath.Join(manifestsPath, "kapp-diff.json"))
}

// checkDiffSupport reports whether the downloaded distribution source has the diff.sh template of the phase. A
// rendered diff.sh in the run folder does not count.
func checkDiffSupport(title, distroPath, phase string) (Section, bool) {
	tpl := filepath.Join(distroPath, "templates", phase, "scripts", "diff.sh.tpl")

	if _, err := os.Stat(tpl); err == nil {
		return Section{}, true
	}

	reason := "distribution " + distributionVersion(distroPath)

	return Section{Title: title, Status: StatusNotSupported, Reason: reason}, false
}

func distributionVersion(distroPath string) string {
	kfd, err := yamlx.FromFileV3[struct {
		Version string `yaml:"version"`
	}](filepath.Join(distroPath, "kfd.yaml"))
	if err != nil || kfd.Version == "" {
		return "(unknown version)"
	}

	if strings.HasPrefix(kfd.Version, "v") {
		return kfd.Version
	}

	return "v" + kfd.Version
}

// runDiffScript deletes the kapp output of earlier runs, so a stale result is never read, and runs the script.
func runDiffScript(r CaptureRunner, script, kappDir string) (string, int, error) {
	stale, err := filepath.Glob(filepath.Join(kappDir, "kapp-diff*.json"))
	if err != nil {
		return "", 0, fmt.Errorf("error listing earlier kapp output: %w", err)
	}

	for _, f := range stale {
		if err := os.Remove(f); err != nil {
			return "", 0, fmt.Errorf("error removing earlier kapp output: %w", err)
		}
	}

	out, code, err := r.RunCapture(script)
	if err != nil {
		return "", 0, fmt.Errorf("scripts/diff.sh could not run: %w", err)
	}

	return out, code, nil
}

func failedScript(title string, code int, out, kappDir string) Section {
	return Section{
		Title:  title,
		Status: StatusFailed,
		Reason: fmt.Sprintf("scripts/diff.sh exited %d: %s", code, failureHint(out, kappDir)),
	}
}

// failureHint is the kapp error of the first kapp-diff*.json that holds one, else the last non-empty output line.
func failureHint(out, kappDir string) string {
	if files, err := filepath.Glob(filepath.Join(kappDir, "kapp-diff*.json")); err == nil {
		for _, f := range files {
			// ParseDiff returns the kapp error it found together with ErrInvalidDiff.
			res, err := kapp.ParseDiff(f)
			if err != nil && res.KappError == "" {
				continue
			}

			if res.KappError != "" {
				return res.KappError
			}
		}
	}

	hint := ""

	for line := range strings.SplitSeq(out, "\n") {
		if strings.TrimSpace(line) != "" {
			hint = strings.TrimSpace(line)
		}
	}

	return hint
}

// kappSection reads the kapp output of a script that exited 0.
func kappSection(title, file string) Section {
	res, err := kapp.ParseDiff(file)
	if err != nil {
		return Section{Title: title, Status: StatusFailed, Reason: ReasonKappFormat}
	}

	return comparedSection(title, res)
}

// cleanKappSection reads the kapp output of a plugin of a script that failed: only a valid file without a kapp
// error written by this run is a result.
func cleanKappSection(title, file string) (Section, bool) {
	res, err := kapp.ParseDiff(file)
	if err != nil || res.KappError != "" {
		return Section{}, false
	}

	return comparedSection(title, res), true
}

func comparedSection(title string, res kapp.DiffResult) Section {
	section := Section{Title: title, Status: StatusCompared, Target: res.Target}

	for _, row := range res.Changes() {
		section.Rows = append(section.Rows, Row{Namespace: row.Namespace, Name: row.Name, Kind: row.Kind, Op: row.Op})
	}

	return section
}

// NewDistributionDiffer returns the distribution diff of a cluster whose phases are under paths.WorkDir.
func NewDistributionDiffer(paths cluster.CreatorPaths) func() Section {
	phasePath := path.Join(paths.WorkDir, cluster.OperationPhaseDistribution)

	runner := shell.NewRunner(execx.NewStdExecutor(), shell.Paths{
		Shell:   "sh",
		WorkDir: path.Join(phasePath, "manifests"),
	})

	return func() Section {
		return DiffDistribution(paths.DistroPath, phasePath, runner)
	}
}
