// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package kustomize

import (
	"github.com/sighupio/furyctl/internal/execx"
)

type Paths struct {
	Kustomize string
	WorkDir   string
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
	return execx.CombinedOutput(execx.NewCmd(r.paths.Kustomize, execx.CmdOptions{
		Args:     []string{"version", "--short"},
		Executor: r.executor,
		WorkDir:  r.paths.WorkDir,
	}))
}
