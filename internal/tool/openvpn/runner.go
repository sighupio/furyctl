// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package openvpn

import (
	"fmt"

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
	cmd      *execx.Cmd
}

func NewRunner(executor execx.Executor, paths Paths) *Runner {
	return &Runner{
		executor: executor,
		paths:    paths,
		cmd: execx.NewCmd(paths.Openvpn, execx.CmdOptions{
			Executor: executor,
			WorkDir:  paths.WorkDir,
		}),
	}
}

func (r *Runner) CmdPath() string {
	return r.paths.Openvpn
}

func (r *Runner) Connect(name string) error {
	path := "sudo"
	args := []string{r.paths.Openvpn, "--config", fmt.Sprintf("%s.ovpn", name), "--daemon"}

	userIsRoot, err := osx.IsRoot()
	if err != nil {
		return fmt.Errorf("error while checking if user is root: %w", err)
	}

	if userIsRoot {
		path = r.paths.Openvpn
		args = args[1:]
	}

	r.cmd.Args = args
	r.cmd.Path = path

	if err := r.cmd.Run(); err != nil {
		return fmt.Errorf("error while running openvpn: %w", err)
	}

	return nil
}

func (r *Runner) Version() (string, error) {
	args := []string{r.paths.Openvpn, "--version"}

	r.cmd.Args = args

	out, err := execx.CombinedOutput(r.cmd)
	if err != nil {
		return "", fmt.Errorf("error getting openvpn version: %w", err)
	}

	return out, nil
}

func (r *Runner) Stop() error {
	if err := r.cmd.Stop(); err != nil {
		return fmt.Errorf("error stopping openvpn runner: %w", err)
	}

	return nil
}
