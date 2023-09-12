// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package helmfile

import (
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	execx "github.com/sighupio/furyctl/internal/x/exec"
)

type Paths struct {
	Helmfile   string
	WorkDir    string
	PluginsDir string
}

type Runner struct {
	executor execx.Executor
	paths    Paths
	cmds     map[string]*execx.Cmd
}

func NewRunner(executor execx.Executor, paths Paths) *Runner {
	if paths.PluginsDir != "" {
		if err := os.Setenv("HELM_PLUGINS", paths.PluginsDir); err != nil {
			logrus.Fatal(err)
		}
	}

	return &Runner{
		executor: executor,
		paths:    paths,
		cmds:     make(map[string]*execx.Cmd),
	}
}

func (r *Runner) CmdPath() string {
	return r.paths.Helmfile
}

func (r *Runner) newCmd(args []string) (*execx.Cmd, string) {
	cmd := execx.NewCmd(r.paths.Helmfile, execx.CmdOptions{
		Args:     args,
		Executor: r.executor,
		WorkDir:  r.paths.WorkDir,
	})

	id := uuid.NewString()
	r.cmds[id] = cmd

	return cmd, id
}

func (r *Runner) deleteCmd(id string) {
	delete(r.cmds, id)
}

func (r *Runner) Init() error {
	args := []string{"init", "--force"}

	cmd, id := r.newCmd(args)
	defer r.deleteCmd(id)

	if _, err := execx.CombinedOutput(cmd); err != nil {
		return fmt.Errorf("error running helmfile init: %w", err)
	}

	return nil
}

func (r *Runner) Apply() error {
	args := []string{"apply"}

	cmd, id := r.newCmd(args)
	defer r.deleteCmd(id)

	if _, err := execx.CombinedOutput(cmd); err != nil {
		return fmt.Errorf("error running helmfile apply: %w", err)
	}

	return nil
}

func (r *Runner) Version() (string, error) {
	cmd, id := r.newCmd([]string{"version", "-o=short"})
	defer r.deleteCmd(id)

	out, err := execx.CombinedOutput(cmd)
	if err != nil {
		return "", fmt.Errorf("error getting helmfile version: %w", err)
	}

	return out, nil
}

func (r *Runner) Stop() error {
	for _, cmd := range r.cmds {
		if err := cmd.Stop(); err != nil {
			return fmt.Errorf("error stopping helmfile runner: %w", err)
		}
	}

	return nil
}
