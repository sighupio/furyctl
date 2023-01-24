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
	executor      execx.Executor
	paths         Paths
	serverSide    bool
	skipNotFound  bool
	clientVersion bool
}

func NewRunner(executor execx.Executor, paths Paths, serverSide, skipNotFound, clientVersion bool) *Runner {
	return &Runner{
		executor:      executor,
		paths:         paths,
		serverSide:    serverSide,
		skipNotFound:  skipNotFound,
		clientVersion: clientVersion,
	}
}

func (r *Runner) CmdPath() string {
	return r.paths.Kubectl
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

func (r *Runner) Delete(manifestPath string, params ...string) error {
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
	args := []string{"version"}

	if r.paths.Kubeconfig != "" {
		args = append(args, "--kubeconfig", r.paths.Kubeconfig)
	}

	if r.clientVersion {
		args = append(args, "--client")
	}

	out, err := execx.CombinedOutput(execx.NewCmd(r.paths.Kubectl, execx.CmdOptions{
		Args:     args,
		Executor: r.executor,
	}))
	if err != nil {
		return "", fmt.Errorf("error getting kubectl version: %w", err)
	}

	return out, nil
}
