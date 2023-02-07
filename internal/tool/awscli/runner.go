// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package awscli

import (
	"fmt"

	execx "github.com/sighupio/furyctl/internal/x/exec"
)

type Paths struct {
	Awscli  string
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
	return r.paths.Awscli
}

func (r *Runner) Ec2(sub string, params ...string) (string, error) {
	args := []string{"ec2", sub}

	if len(params) > 0 {
		args = append(args, params...)
	}

	out, err := execx.CombinedOutput(execx.NewCmd(r.paths.Awscli, execx.CmdOptions{
		Args:     args,
		Executor: r.executor,
		WorkDir:  r.paths.WorkDir,
	}))
	if err != nil {
		return "", fmt.Errorf("error running awscli ec2 %s: %w", sub, err)
	}

	return out, nil
}

func (r *Runner) Version() (string, error) {
	out, err := execx.CombinedOutput(execx.NewCmd(r.paths.Awscli, execx.CmdOptions{
		Args:     []string{"--version"},
		Executor: r.executor,
		WorkDir:  r.paths.WorkDir,
	}))
	if err != nil {
		return "", fmt.Errorf("error getting awscli version: %w", err)
	}

	return out, nil
}
