// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sed

import (
	"fmt"

	"github.com/google/uuid"

	execx "github.com/sighupio/furyctl/internal/x/exec"
)

type Paths struct {
	Sed     string
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
	return r.paths.Sed
}

func (r *Runner) newCmd(args []string) (*execx.Cmd, string) {
	cmd := execx.NewCmd(r.paths.Sed, execx.CmdOptions{
		Args:     args,
		Executor: r.executor,
		WorkDir:  r.paths.WorkDir,
	})

	id := uuid.NewString()
	r.cmds[id] = cmd

	return cmd, id
}

func (r *Runner) deleteCmd(id string) {
	delete(r.cmds, id)
}

func (r *Runner) Version() (string, error) {
	cmd, id := r.newCmd([]string{""})
	defer r.deleteCmd(id)

	_, err := execx.CombinedOutput(cmd)
	if err != nil {
		return "", fmt.Errorf("error getting sed version: %w", err)
	}

	return "v0.0.0", nil
}

func (r *Runner) Run(args ...string) (string, error) {
	cmd, id := r.newCmd(args)
	defer r.deleteCmd(id)

	out, err := execx.CombinedOutput(cmd)
	if err != nil {
		return "", fmt.Errorf("error while running sed: %w", err)
	}

	return out, nil
}

func (r *Runner) Stop() error {
	for _, cmd := range r.cmds {
		if err := cmd.Stop(); err != nil {
			return fmt.Errorf("error stopping sed runner: %w", err)
		}
	}

	return nil
}
