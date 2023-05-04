// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package git

import (
	"fmt"

	execx "github.com/sighupio/furyctl/internal/x/exec"
)

type Paths struct {
	Git     string
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
	return r.paths.Git
}

func (r *Runner) Version() (string, error) {
	out, err := execx.CombinedOutput(execx.NewCmd(r.paths.Git, execx.CmdOptions{
		Args:     []string{"version"},
		Executor: r.executor,
		WorkDir:  r.paths.WorkDir,
	}))
	if err != nil {
		return "", fmt.Errorf("error getting git version: %w", err)
	}

	return out, nil
}