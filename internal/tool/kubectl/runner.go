// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package kubectl

import (
	"fmt"

	"github.com/google/uuid"

	execx "github.com/sighupio/furyctl/internal/x/exec"
)

type Paths struct {
	Kubectl    string
	WorkDir    string
	Kubeconfig string
}

type Runner struct {
	executor      execx.Executor
	paths         Paths
	serverSide    bool
	skipNotFound  bool
	clientVersion bool
	cmds          map[string]*execx.Cmd
}

func NewRunner(executor execx.Executor, paths Paths, serverSide, skipNotFound, clientVersion bool) *Runner {
	return &Runner{
		executor:      executor,
		paths:         paths,
		serverSide:    serverSide,
		skipNotFound:  skipNotFound,
		clientVersion: clientVersion,
		cmds:          make(map[string]*execx.Cmd),
	}
}

func (r *Runner) CmdPath() string {
	return r.paths.Kubectl
}

func (r *Runner) newCmd(args []string, sensitive bool) (*execx.Cmd, string) {
	cmd := execx.NewCmd(r.paths.Kubectl, execx.CmdOptions{
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

func (r *Runner) Apply(manifestPath string, params ...string) error {
	args := []string{"apply"}

	if r.paths.Kubeconfig != "" {
		args = append(args, "--kubeconfig", r.paths.Kubeconfig)
	}

	if r.serverSide {
		args = append(args, "--server-side")
	}

	if len(params) > 0 {
		args = append(args, params...)
	}

	args = append(args, "-f", manifestPath)

	cmd, id := r.newCmd(args, false)
	defer r.deleteCmd(id)

	if _, err := execx.CombinedOutput(cmd); err != nil {
		return fmt.Errorf("error applying manifests: %w", err)
	}

	return nil
}

func (r *Runner) Get(sensitive bool, ns string, params ...string) (string, error) {
	args := []string{"get"}

	if r.paths.Kubeconfig != "" {
		args = append(args, "--kubeconfig", r.paths.Kubeconfig)
	}

	if ns != "all" {
		args = append(args, "-n", ns)
	} else {
		args = append(args, "-A")
	}

	args = append(args, params...)

	cmd, id := r.newCmd(args, sensitive)
	defer r.deleteCmd(id)

	out, err := execx.CombinedOutput(cmd)
	if err != nil {
		return out, fmt.Errorf("error while getting resources: %w", err)
	}

	return out, nil
}

func (r *Runner) Version() (string, error) {
	args := []string{"version"}

	if r.paths.Kubeconfig != "" {
		args = append(args, "--kubeconfig", r.paths.Kubeconfig)
	}

	if r.clientVersion {
		args = append(args, "--client")
	}

	args = append(args, "-o", "json")

	cmd, id := r.newCmd(args, false)
	defer r.deleteCmd(id)

	out, err := execx.CombinedOutput(cmd)
	if err != nil {
		return "", fmt.Errorf("error getting kubectl version: %w", err)
	}

	return out, nil
}

func (r *Runner) Stop() error {
	for _, cmd := range r.cmds {
		if err := cmd.Stop(); err != nil {
			return fmt.Errorf("error stopping kubectl runner: %w", err)
		}
	}

	return nil
}
