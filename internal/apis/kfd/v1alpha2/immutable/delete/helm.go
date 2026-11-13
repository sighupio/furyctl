// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package del

import (
	"bytes"
	"fmt"
	"strings"

	execx "github.com/sighupio/furyctl/internal/x/exec"
)

// HelmRunner reads what a helm release installed.
type HelmRunner interface {
	GetManifest(release, namespace string) (string, error)
	ShowCRDs(chart, version string) (string, error)
}

// helmCLI runs the helm reads that helm.Runner does not have.
type helmCLI struct {
	executor execx.Executor
	helm     string
	workDir  string
}

func (h helmCLI) GetManifest(release, namespace string) (string, error) {
	return h.output(withFlag([]string{"get", "manifest", release}, "-n", namespace))
}

func (h helmCLI) ShowCRDs(chart, version string) (string, error) {
	return h.output(withFlag([]string{"show", "crds", chart}, "--version", version))
}

// withFlag leaves out a flag without value: helm then uses its default, the default namespace or the latest chart.
func withFlag(args []string, flag, value string) []string {
	if value == "" {
		return args
	}

	return append(args, flag, value)
}

func (h helmCLI) output(args []string) (string, error) {
	// Sensitive: manifests hold Secrets, so nothing reaches the terminal or the logs.
	cmd := execx.NewCmd(h.helm, execx.CmdOptions{Args: args, Executor: h.executor, WorkDir: h.workDir, Sensitive: true})

	out, okOut := cmd.Stdout.(*bytes.Buffer)
	errOut, okErr := cmd.Stderr.(*bytes.Buffer)

	if !okOut || !okErr {
		return "", execx.ErrCastingToBuffer
	}

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("error running helm %s: %w: %s",
			strings.Join(args, " "), err, strings.TrimSpace(errOut.String()))
	}

	return out.String(), nil
}
