// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package kapp

import (
	"fmt"

	"github.com/google/uuid"

	execx "github.com/sighupio/furyctl/internal/x/exec"
)

type Paths struct {
	Kapp    string
	WorkDir string
}

type Runner struct {
	executor   execx.Executor
	paths      Paths
	serverSide bool
	cmds       map[string]*execx.Cmd
}

func NewRunner(executor execx.Executor, paths Paths, serverSide bool) *Runner {
	return &Runner{
		executor:   executor,
		paths:      paths,
		serverSide: serverSide,
		cmds:       make(map[string]*execx.Cmd),
	}
}

func (r *Runner) CmdPath() string {
	return r.paths.Kapp
}

func (r *Runner) newCmd(args []string, sensitive bool) (*execx.Cmd, string) {
	cmd := execx.NewCmd(r.paths.Kapp, execx.CmdOptions{
		Args:      args,
		Executor:  r.executor,
		WorkDir:   r.paths.WorkDir,
		Sensitive: sensitive,
	})

	id := uuid.NewString()
	r.cmds[id] = cmd

	return cmd, id
}

func (r *Runner) deleteCmd(id string) {
	delete(r.cmds, id)
}

func (r *Runner) Deploy(manifestPath string, params ...string) error {
	args := []string{"deploy"}

	if r.serverSide {
		args = append(args, "--server-side")
	}

	if len(params) > 0 {
		args = append(args, params...)
	}

	args = append(args, "-a", "kfd")

	args = append(args, "-n", "kube-system")

	args = append(args, "-f", manifestPath)

	cmd, id := r.newCmd(args, false)
	defer r.deleteCmd(id)

	if _, err := execx.CombinedOutput(cmd); err != nil {
		return fmt.Errorf("error applying manifests: %w", err)
	}

	return nil
}

func (r *Runner) Version() (string, error) {
	args := []string{"version"}

	cmd, id := r.newCmd(args, false)
	defer r.deleteCmd(id)

	out, err := execx.CombinedOutput(cmd)
	if err != nil {
		return "", fmt.Errorf("error getting kapp version: %w", err)
	}

	return out, nil
}

func (r *Runner) Stop() error {
	for _, cmd := range r.cmds {
		if err := cmd.Stop(); err != nil {
			return fmt.Errorf("error stopping kapp runner: %w", err)
		}
	}

	return nil
}
