// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package furyagent

import (
	"bytes"
	"fmt"

	execx "github.com/sighupio/furyctl/internal/x/exec"
)

type Paths struct {
	Furyagent string
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
		cmd: execx.NewCmd(paths.Furyagent, execx.CmdOptions{
			Executor: executor,
			WorkDir:  paths.WorkDir,
		}),
	}
}

func (r *Runner) CmdPath() string {
	return r.paths.Furyagent
}

func (r *Runner) ConfigOpenvpnClient(name string, params ...string) (*bytes.Buffer, error) {
	args := []string{
		r.paths.Furyagent,
		"configure",
		"openvpn-client",
		fmt.Sprintf("--client-name=%s", name),
		"--config=furyagent.yml",
	}

	r.cmd.Args = args

	if err := r.cmd.Run(); err != nil {
		return nil, fmt.Errorf("error while running furyagent configure openvpn-client: %w", err)
	}

	return r.cmd.Log.Out, nil
}

func (r *Runner) Version() (string, error) {
	args := []string{r.paths.Furyagent, "version"}

	r.cmd.Args = args

	out, err := execx.CombinedOutput(r.cmd)
	if err != nil {
		return "", fmt.Errorf("error getting furyagent version: %w", err)
	}

	return out, nil
}

func (r *Runner) Stop() error {
	if err := r.cmd.Stop(); err != nil {
		return fmt.Errorf("error stopping furyagent runner: %w", err)
	}

	return nil
}
