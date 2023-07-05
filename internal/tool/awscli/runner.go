// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package awscli

import (
	"fmt"

	"github.com/google/uuid"

	execx "github.com/sighupio/furyctl/internal/x/exec"
)

type Paths struct {
	Awscli  string
	WorkDir string
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
	return r.paths.Awscli
}

func (r *Runner) newCmd(args []string) (*execx.Cmd, string) {
	cmd := execx.NewCmd(r.paths.Awscli, execx.CmdOptions{
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

func (r *Runner) Ec2(sub string, params ...string) (string, error) {
	args := []string{"ec2", sub}

	if len(params) > 0 {
		args = append(args, params...)
	}

	cmd, id := r.newCmd(args)
	defer r.deleteCmd(id)

	out, err := execx.CombinedOutput(cmd)
	if err != nil {
		return "", fmt.Errorf("error running awscli ec2 %s: %w", sub, err)
	}

	return out, nil
}

func (r *Runner) S3(params ...string) (string, error) {
	args := []string{"s3"}
	args = append(args, params...)

	cmd, id := r.newCmd(args)
	defer r.deleteCmd(id)

	out, err := execx.CombinedOutput(cmd)
	if err != nil {
		return "", fmt.Errorf("error executing awscli s3: %w", err)
	}

	return out, nil
}

func (r *Runner) S3Api(params ...string) (string, error) {
	args := []string{"s3api"}
	args = append(args, params...)

	cmd, id := r.newCmd(args)
	defer r.deleteCmd(id)

	out, err := execx.CombinedOutput(cmd)
	if err != nil {
		return "", fmt.Errorf("error executing awscli s3api: %w", err)
	}

	return out, nil
}

func (r *Runner) Route53(sub string, params ...string) (string, error) {
	args := []string{"route53", sub}

	if len(params) > 0 {
		args = append(args, params...)
	}

	cmd, id := r.newCmd(args)
	defer r.deleteCmd(id)

	out, err := execx.CombinedOutput(cmd)
	if err != nil {
		return "", fmt.Errorf("error running awscli ec2 %s: %w", sub, err)
	}

	return out, nil
}

func (r *Runner) Version() (string, error) {
	args := []string{"--version"}

	cmd, id := r.newCmd(args)
	defer r.deleteCmd(id)

	out, err := execx.CombinedOutput(cmd)
	if err != nil {
		return "", fmt.Errorf("error getting awscli version: %w", err)
	}

	return out, nil
}

func (r *Runner) Stop() error {
	for _, cmd := range r.cmds {
		if err := cmd.Stop(); err != nil {
			return fmt.Errorf("error stopping awscli runner: %w", err)
		}
	}

	return nil
}
