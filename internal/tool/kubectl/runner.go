// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package kubectl

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	execx "github.com/sighupio/furyctl/internal/x/exec"
)

const (
	kubectlDeleteTimeout = 5 * time.Minute
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

func (r *Runner) newCmd(args []string) (*execx.Cmd, string) {
	cmd := execx.NewCmd(r.paths.Kubectl, execx.CmdOptions{
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

	cmd, id := r.newCmd(args)
	defer r.deleteCmd(id)

	if _, err := execx.CombinedOutput(cmd); err != nil {
		return fmt.Errorf("error applying manifests: %w", err)
	}

	return nil
}

func (r *Runner) Get(ns string, params ...string) (string, error) {
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

	cmd, id := r.newCmd(args)
	defer r.deleteCmd(id)

	out, err := execx.CombinedOutput(cmd)
	if err != nil {
		return out, fmt.Errorf("error while getting resources: %w", err)
	}

	return out, nil
}

func (r *Runner) APIResources(params ...string) (string, error) {
	args := []string{"api-resources"}

	if r.paths.Kubeconfig != "" {
		args = append(args, "--kubeconfig", r.paths.Kubeconfig)
	}

	args = append(args, params...)

	cmd, id := r.newCmd(args)
	defer r.deleteCmd(id)

	out, err := execx.CombinedOutput(cmd)
	if err != nil {
		return out, fmt.Errorf("error while listing api resources: %w", err)
	}

	return out, nil
}

func (r *Runner) GetResource(ns, res, name string) (string, error) {
	args := []string{"get", res, "-n", ns, name}

	if r.paths.Kubeconfig != "" {
		args = append(args, "--kubeconfig", r.paths.Kubeconfig)
	}

	cmd, id := r.newCmd(args)
	defer r.deleteCmd(id)

	out, err := execx.CombinedOutput(cmd)
	if err != nil {
		return out, fmt.Errorf("error while getting resources: %w", err)
	}

	return out, nil
}

// DeleteResource deletes the specified resource in the specified namespace.
func (r *Runner) DeleteResource(ns, res, name string) (string, error) {
	args := []string{"delete", "--namespace", ns, res, name}

	if r.paths.Kubeconfig != "" {
		args = append(args, "--kubeconfig", r.paths.Kubeconfig)
	}

	cmd, id := r.newCmd(args)
	defer r.deleteCmd(id)

	out, err := execx.CombinedOutput(cmd)
	if err != nil {
		return out, fmt.Errorf("error deleting resource(s) \"%s/%s/%s\": %w", ns, res, name, err)
	}

	return out, nil
}

// DeleteResources deletes the specified resources in the specified namespace.
func (r *Runner) DeleteResources(ns, res string) (string, error) {
	args := []string{"delete", "--namespace", ns, "--all", res}

	if r.paths.Kubeconfig != "" {
		args = append(args, "--kubeconfig", r.paths.Kubeconfig)
	}

	cmd, id := r.newCmd(args)
	defer r.deleteCmd(id)

	out, err := execx.CombinedOutput(cmd)
	if err != nil {
		return out, fmt.Errorf("error deleting resource(s) \"%s/%s\": %w", ns, res, err)
	}

	return out, nil
}

// DeleteResourcesInAllNamespaces deletes the specified resources in all namespaces.
func (r *Runner) DeleteResourcesInAllNamespaces(res string) (string, error) {
	args := []string{"delete", "--all-namespaces", "--all", res}

	if r.paths.Kubeconfig != "" {
		args = append(args, "--kubeconfig", r.paths.Kubeconfig)
	}

	cmd, id := r.newCmd(args)
	defer r.deleteCmd(id)

	out, err := execx.CombinedOutput(cmd)
	if err != nil {
		return out, fmt.Errorf("error deleting all \"%s\" resources in all namespaces: %w", res, err)
	}

	return out, nil
}

func (r *Runner) Delete(manifestPath string, params ...string) (string, error) {
	args := []string{"delete"}

	if r.paths.Kubeconfig != "" {
		args = append(args, "--kubeconfig", r.paths.Kubeconfig)
	}

	if r.skipNotFound {
		args = append(args, "--ignore-not-found=true")
	}

	if len(params) > 0 {
		args = append(args, params...)
	}

	args = append(args, "-f", manifestPath)

	cmd, id := r.newCmd(args)
	defer r.deleteCmd(id)

	out, err := execx.CombinedOutputWithTimeout(cmd, kubectlDeleteTimeout)
	if err != nil {
		return out, fmt.Errorf("error deleting resources: %w", err)
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

	cmd, id := r.newCmd(args)
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
