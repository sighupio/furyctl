// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package golang

import (
	"fmt"

	"github.com/google/uuid"

	execx "github.com/sighupio/furyctl/internal/x/exec"
)

type Paths struct {
	Go      string
	WorkDir string
}

type Runner struct {
	executor execx.Executor
	paths    Paths
	cmds     map[string]*execx.Cmd
}

func NewRunner(executor execx.Executor, paths Paths) *Runner {
	return &Runner{
		executor: executor,
		paths:    paths,
		cmds:     make(map[string]*execx.Cmd),
	}
}

func (r *Runner) CmdPath() string {
	return r.paths.Go
}

func (r *Runner) deleteCmd(id string) {
	delete(r.cmds, id)
}

func (r *Runner) newCmd(args []string, sensitive bool) (*execx.Cmd, string) {
	cmd := execx.NewCmd(r.paths.Go, execx.CmdOptions{
		Args:      args,
		Executor:  r.executor,
		WorkDir:   r.paths.WorkDir,
		Sensitive: sensitive,
	})

	id := uuid.NewString()
	r.cmds[id] = cmd

	return cmd, id
}

func (r *Runner) Test(manifestPath string, params ...string) error {
	args := []string{"test"}

	if len(params) > 0 {
		args = append(args, params...)
	}

	args = append(args, "-o", "json")

	cmd, id := r.newCmd(args, false)
	defer r.deleteCmd(id)

	if _, err := execx.CombinedOutput(cmd); err != nil {
		return fmt.Errorf("error running tests: %w", err)
	}

	return nil
}

func (r *Runner) Version() (string, error) {
	args := []string{"version"}

	cmd, id := r.newCmd(args, false)
	defer r.deleteCmd(id)

	out, err := execx.CombinedOutput(cmd)
	if err != nil {
		return "", fmt.Errorf("error getting go version: %w", err)
	}

	return out, nil
}

func (r *Runner) Stop() error {
	for _, cmd := range r.cmds {
		if err := cmd.Stop(); err != nil {
			return fmt.Errorf("error stopping go runner: %w", err)
		}
	}

	return nil
}
