// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package openvpn

import (
	"fmt"

	"github.com/sighupio/furyctl/internal/execx"
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

func (r *Runner) Connect(name string) error {
	return execx.NewCmd(r.paths.Openvpn, execx.CmdOptions{
		Args:     []string{"--config", fmt.Sprintf("%s.ovpn", name), "--daemon"},
		Executor: r.executor,
		WorkDir:  r.paths.WorkDir,
	}).Run()
}

func (r *Runner) Version() (string, error) {
	return execx.CombinedOutput(execx.NewCmd(r.paths.Openvpn, execx.CmdOptions{
		Args:     []string{"--version"},
		Executor: r.executor,
		WorkDir:  r.paths.WorkDir,
	}))
}
