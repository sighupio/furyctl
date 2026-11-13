// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package kapp

import (
	"bytes"
	"fmt"

	"github.com/google/uuid"

	execx "github.com/sighupio/furyctl/internal/x/exec"
)

type Paths struct {
	Kapp    string
	WorkDir string
}

type Runner struct {
	executor   execx.Executor
	paths      Paths
	serverSide bool
	cmds       map[string]*execx.Cmd
}

func NewRunner(executor execx.Executor, paths Paths, serverSide bool) *Runner {
	return &Runner{
		executor:   executor,
		paths:      paths,
		serverSide: serverSide,
		cmds:       make(map[string]*execx.Cmd),
	}
}

func (r *Runner) CmdPath() string {
	return r.paths.Kapp
}

func (r *Runner) Deploy(manifestPath string, params ...string) error {
	args := []string{"deploy"}

	if r.serverSide {
		args = append(args, "--server-side")
	}

	if len(params) > 0 {
		args = append(args, params...)
	}

	args = append(args, "-a", "kfd")

	args = append(args, "-n", "kube-system")

	args = append(args, "-f", manifestPath)

	cmd, id := r.newCmd(args, false)
	defer r.deleteCmd(id)

	if _, err := execx.CombinedOutput(cmd); err != nil {
		return fmt.Errorf("error applying manifests: %w", err)
	}

	return nil
}

// Delete deletes every object of the app in the order of kapp's built-in delete rules.
func (r *Runner) Delete(app string, params ...string) error {
	args := append([]string{"delete", "-a", app, "-n", "kube-system", "-y"}, params...)

	cmd, id := r.newCmd(args, false)
	defer r.deleteCmd(id)

	if _, err := execx.CombinedOutput(cmd); err != nil {
		return fmt.Errorf("error deleting app %s: %w", app, err)
	}

	return nil
}

// Inspect returns the standard output of kapp inspect for the app.
func (r *Runner) Inspect(app string, params ...string) (string, error) {
	args := append([]string{"inspect", "-a", app, "-n", "kube-system"}, params...)

	// Sensitive: --raw prints whole objects, Secrets included, so nothing reaches the terminal or the logs.
	cmd, id := r.newCmd(args, true)
	defer r.deleteCmd(id)

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("error inspecting app %s: %w", app, err)
	}

	out, ok := cmd.Stdout.(*bytes.Buffer)
	if !ok {
		return "", execx.ErrCastingToBuffer
	}

	return out.String(), nil
}

func (r *Runner) Version() (string, error) {
	args := []string{"version"}

	cmd, id := r.newCmd(args, false)
	defer r.deleteCmd(id)

	out, err := execx.CombinedOutput(cmd)
	if err != nil {
		return "", fmt.Errorf("error getting kapp version: %w", err)
	}

	return out, nil
}

func (r *Runner) Stop() error {
	for _, cmd := range r.cmds {
		if err := cmd.Stop(); err != nil {
			return fmt.Errorf("error stopping kapp runner: %w", err)
		}
	}

	return nil
}

func (r *Runner) newCmd(args []string, sensitive bool) (*execx.Cmd, string) {
	cmd := execx.NewCmd(r.paths.Kapp, execx.CmdOptions{
		Args:      args,
		Executor:  r.executor,
		WorkDir:   r.paths.WorkDir,
		Sensitive: sensitive,
	})

	id := uuid.NewString()
	r.cmds[id] = cmd

	return cmd, id
}

func (r *Runner) deleteCmd(id string) {
	delete(r.cmds, id)
}
