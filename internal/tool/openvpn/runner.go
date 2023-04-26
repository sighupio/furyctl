// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package openvpn

import (
	"fmt"

	"github.com/google/uuid"

	execx "github.com/sighupio/furyctl/internal/x/exec"
	osx "github.com/sighupio/furyctl/internal/x/os"
)

type Paths struct {
	Openvpn string
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
	}
}

func (r *Runner) CmdPath() string {
	return r.paths.Openvpn
}

func (r *Runner) newCmdWithPath(path string, args []string) (*execx.Cmd, string) {
	cmd := execx.NewCmd(path, execx.CmdOptions{
		Args:     args,
		Executor: r.executor,
		WorkDir:  r.paths.WorkDir,
	})

	id := uuid.NewString()
	r.cmds[id] = cmd

	return cmd, id
}

func (r *Runner) newCmd(args []string) (*execx.Cmd, string) {
	return r.newCmdWithPath(r.paths.Openvpn, args)
}

func (r *Runner) deleteCmd(id string) {
	delete(r.cmds, id)
}

func (r *Runner) Connect(name string) error {
	path := "sudo"
	args := []string{"--config", fmt.Sprintf("%s.ovpn", name), "--daemon"}

	userIsRoot, err := osx.IsRoot()
	if err != nil {
		return fmt.Errorf("error while checking if user is root: %w", err)
	}

	if userIsRoot {
		path = r.paths.Openvpn
		args = args[1:]
	}

	cmd, id := r.newCmdWithPath(path, args)
	defer r.deleteCmd(id)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error while running openvpn: %w", err)
	}

	return nil
}

func (r *Runner) Version() (string, error) {
	args := []string{"--version"}

	cmd, id := r.newCmd(args)
	defer r.deleteCmd(id)

	out, err := execx.CombinedOutput(cmd)
	if err != nil {
		return "", fmt.Errorf("error getting openvpn version: %w", err)
	}

	return out, nil
}

func (r *Runner) Stop() error {
	for _, cmd := range r.cmds {
		if err := cmd.Stop(); err != nil {
			return fmt.Errorf("error stopping openvpn runner: %w", err)
		}
	}

	return nil
}
