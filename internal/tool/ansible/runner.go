// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ansible

import (
	"fmt"

	execx "github.com/sighupio/furyctl/internal/x/exec"
)

type Paths struct {
	Ansible string
	WorkDir string
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
		cmd: execx.NewCmd(paths.Ansible, execx.CmdOptions{
			Executor: executor,
			WorkDir:  paths.WorkDir,
		}),
	}
}

func (r *Runner) CmdPath() string {
	return r.paths.Ansible
}

func (r *Runner) Version() (string, error) {
	args := []string{r.paths.Ansible, "--version"}

	r.cmd.Args = args

	out, err := execx.CombinedOutput(r.cmd)
	if err != nil {
		return "", fmt.Errorf("error getting ansible version: %w", err)
	}

	return out, nil
}

func (r *Runner) Stop() error {
	if err := r.cmd.Stop(); err != nil {
		return fmt.Errorf("error stopping ansible runner: %w", err)
	}

	return nil
}
