// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package kustomize

import (
	"fmt"

	execx "github.com/sighupio/furyctl/internal/x/exec"
)

type Paths struct {
	Kustomize string
	WorkDir   string
}

type Runner struct {
	executor execx.Executor
	paths    Paths
	cmd      *execx.Cmd
}

func NewRunner(executor execx.Executor, paths Paths) *Runner {
	return &Runner{
		executor: executor,
		paths:    paths,
		cmd: execx.NewCmd(paths.Kustomize, execx.CmdOptions{
			Executor: executor,
			WorkDir:  paths.WorkDir,
		}),
	}
}

func (r *Runner) CmdPath() string {
	return r.paths.Kustomize
}

func (r *Runner) Version() (string, error) {
	args := []string{r.paths.Kustomize, "version", "--short"}

	r.cmd.Args = args

	out, err := execx.CombinedOutput(r.cmd)
	if err != nil {
		return "", fmt.Errorf("error getting kustomize version: %w", err)
	}

	return out, nil
}

func (r *Runner) Build() (string, error) {
	args := []string{r.paths.Kustomize, "build", "--load_restrictor", "none", "."}

	r.cmd.Args = args

	out, err := execx.CombinedOutput(r.cmd)
	if err != nil {
		return "", fmt.Errorf("error while running kustomize build: %w", err)
	}

	return out, nil
}

func (r *Runner) Stop() error {
	if err := r.cmd.Stop(); err != nil {
		return fmt.Errorf("error stopping kustomize runner: %w", err)
	}

	return nil
}
