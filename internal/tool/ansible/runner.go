// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ansible

import (
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

type Paths struct {
	Ansible string
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

func (r *Runner) Version() (string, error) {
	return execx.CombinedOutput(execx.NewCmd(r.paths.Ansible, execx.CmdOptions{
		Args:     []string{"--version"},
		Executor: r.executor,
		WorkDir:  r.paths.WorkDir,
	}))
}
