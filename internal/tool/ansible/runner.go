// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ansible

import (
	"fmt"

	"github.com/google/uuid"

	execx "github.com/sighupio/furyctl/internal/x/exec"
)

type Paths struct {
	Ansible         string
	AnsiblePlaybook string
	WorkDir         string
}

type Runner struct {
	executor execx.Executor
	paths    Paths
	cmds     map[string]*execx.Cmd
}

func NewRunner(executor execx.Executor, paths Paths) *Runner {
	return &Runner{
		executor: executor,
		paths:    paths,
		cmds:     make(map[string]*execx.Cmd),
	}
}

func (r *Runner) CmdPath() string {
	return r.paths.Ansible
}

func (r *Runner) newCmd(args []string) (*execx.Cmd, string) {
	cmd := execx.NewCmd(r.paths.Ansible, execx.CmdOptions{
		Args:     args,
		Executor: r.executor,
		WorkDir:  r.paths.WorkDir,
	})

	id := uuid.NewString()
	r.cmds[id] = cmd

	return cmd, id
}

func (r *Runner) newPlaybookCmd(args []string) (*execx.Cmd, string) {
	cmd := execx.NewCmd(r.paths.AnsiblePlaybook, execx.CmdOptions{
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

func (r *Runner) Playbook(params ...string) ([]byte, error) {
	args := []string{}
	out := []byte{}

	if len(params) > 0 {
		args = append(args, params...)
	}

	cmd, id := r.newPlaybookCmd(args)
	defer r.deleteCmd(id)

	if err := cmd.Run(); err != nil {
		return out, fmt.Errorf("command execution failed: %w", err)
	}

	out = cmd.Log.Out.Bytes()

	return out, nil
}

func (r *Runner) Exec(params ...string) ([]byte, error) {
	args := []string{}
	out := []byte{}

	if len(params) > 0 {
		args = append(args, params...)
	}

	cmd, id := r.newCmd(args)
	defer r.deleteCmd(id)

	if err := cmd.Run(); err != nil {
		return out, fmt.Errorf("command execution failed: %w", err)
	}

	out = cmd.Log.Out.Bytes()

	return out, nil
}

func (r *Runner) Version() (string, error) {
	args := []string{"--version"}

	cmd, id := r.newCmd(args)
	defer r.deleteCmd(id)

	out, err := execx.CombinedOutput(cmd)
	if err != nil {
		return "", fmt.Errorf("error getting ansible version: %w", err)
	}

	return out, nil
}

func (r *Runner) Stop() error {
	for _, cmd := range r.cmds {
		if err := cmd.Stop(); err != nil {
			return fmt.Errorf("error stopping ansible runner: %w", err)
		}
	}

	return nil
}
