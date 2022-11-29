// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package openvpn

import (
	"fmt"

	execx "github.com/sighupio/furyctl/internal/x/exec"
)

type Paths struct {
	Openvpn string
	WorkDir string
}

type Runner struct {
	executor execx.Executor
	paths    Paths
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

func (r *Runner) Connect(name string) error {
	err := execx.NewCmd("sudo", execx.CmdOptions{
		Args:     []string{r.paths.Openvpn, "--config", fmt.Sprintf("%s.ovpn", name), "--daemon"},
		Executor: r.executor,
		WorkDir:  r.paths.WorkDir,
	}).Run()
	if err != nil {
		return fmt.Errorf("error while running openvpn: %w", err)
	}

	return nil
}

func (r *Runner) Version() (string, error) {
	out, err := execx.CombinedOutput(execx.NewCmd(r.paths.Openvpn, execx.CmdOptions{
		Args:     []string{"--version"},
		Executor: r.executor,
		WorkDir:  r.paths.WorkDir,
	}))
	if err != nil {
		return "", fmt.Errorf("error getting openvpn version: %w", err)
	}

	return out, nil
}
