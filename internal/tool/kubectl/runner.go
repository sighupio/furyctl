// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package kubectl

import (
	"fmt"

	execx "github.com/sighupio/furyctl/internal/x/exec"
)

type Paths struct {
	Kubectl string
	WorkDir string
}

type Runner struct {
	executor   execx.Executor
	paths      Paths
	serverSide bool
}

func NewRunner(executor execx.Executor, paths Paths, serverSide bool) *Runner {
	return &Runner{
		executor:   executor,
		paths:      paths,
		serverSide: serverSide,
	}
}

func (r *Runner) Apply(manifestPath string) error {
	args := []string{"apply", "-f"}

	if r.serverSide {
		args = append(args, "--server-side")
	}

	args = append(args, manifestPath)

	_, err := execx.CombinedOutput(execx.NewCmd(r.paths.Kubectl, execx.CmdOptions{
		Args:     args,
		Executor: r.executor,
		WorkDir:  r.paths.WorkDir,
	}))
	if err != nil {
		return fmt.Errorf("error applying manifest: %w", err)
	}

	return nil
}

func (r *Runner) Version() (string, error) {
	out, err := execx.CombinedOutput(execx.NewCmd(r.paths.Kubectl, execx.CmdOptions{
		Args:     []string{"version", "--client"},
		Executor: r.executor,
		WorkDir:  r.paths.WorkDir,
	}))
	if err != nil {
		return "", fmt.Errorf("error getting kubectl version: %w", err)
	}

	return out, nil
}
