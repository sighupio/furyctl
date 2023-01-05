// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package kubectl

import (
	"fmt"
	"time"

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
	executor     execx.Executor
	paths        Paths
	serverSide   bool
	skipNotFound bool
}

func NewRunner(executor execx.Executor, paths Paths, serverSide, skipNotFound bool) *Runner {
	return &Runner{
		executor:     executor,
		paths:        paths,
		serverSide:   serverSide,
		skipNotFound: skipNotFound,
	}
}

func (r *Runner) CmdPath() string {
	return r.paths.Kubectl
}

func (r *Runner) Apply(manifestPath string) error {
	args := []string{"apply"}

	if r.paths.Kubeconfig != "" {
		args = append(args, "--kubeconfig", r.paths.Kubeconfig)
	}

	if r.serverSide {
		args = append(args, "--server-side")
	}

	args = append(args, "-f", manifestPath)

	_, err := execx.CombinedOutput(execx.NewCmd(r.paths.Kubectl, execx.CmdOptions{
		Args:     args,
		Executor: r.executor,
		WorkDir:  r.paths.WorkDir,
	}))
	if err != nil {
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

	out, err := execx.CombinedOutput(execx.NewCmd(r.paths.Kubectl, execx.CmdOptions{
		Args:     args,
		Executor: r.executor,
		WorkDir:  r.paths.WorkDir,
	}))
	if err != nil {
		return out, fmt.Errorf("error while getting resources: %w", err)
	}

	return out, nil
}

func (r *Runner) DeleteAllResources(res, ns string) (string, error) {
	args := []string{"delete", res, "--all"}

	if ns != "all" {
		args = append(args, "-n", ns)
	} else {
		args = append(args, "-A")
	}

	if r.paths.Kubeconfig != "" {
		args = append(args, "--kubeconfig", r.paths.Kubeconfig)
	}

	out, err := execx.CombinedOutput(execx.NewCmd(r.paths.Kubectl, execx.CmdOptions{
		Args:     args,
		Executor: r.executor,
		WorkDir:  r.paths.WorkDir,
	}))
	if err != nil {
		return out, fmt.Errorf("error deleting all resources: %w", err)
	}

	return out, nil
}

func (r *Runner) Delete(manifestPath string) error {
	args := []string{"delete"}

	if r.paths.Kubeconfig != "" {
		args = append(args, "--kubeconfig", r.paths.Kubeconfig)
	}

	if r.skipNotFound {
		args = append(args, "--ignore-not-found=true")
	}

	args = append(args, "-f", manifestPath)

	_, err := execx.CombinedOutputWithTimeout(execx.NewCmd(r.paths.Kubectl, execx.CmdOptions{
		Args:     args,
		Executor: r.executor,
		WorkDir:  r.paths.WorkDir,
	}), kubectlDeleteTimeout)
	if err != nil {
		return fmt.Errorf("error deleting resources: %w", err)
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
