// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package terraform

import (
	"encoding/json"
	"fmt"
	"os"
	"path"

	"github.com/google/uuid"
	tfjson "github.com/hashicorp/terraform-json"

	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
)

type OutputJSON map[string]*tfjson.StateOutput

type Paths struct {
	Logs      string
	Outputs   string
	Plan      string
	Terraform string
	WorkDir   string
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
	return r.paths.Terraform
}

func (r *Runner) newCmd(args []string) (*execx.Cmd, string) {
	cmd := execx.NewCmd(r.paths.Terraform, execx.CmdOptions{
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
	args := []string{"init"}

	if execx.NoTTY {
		args = append(args, "-no-color")
	}

	cmd, id := r.newCmd(args)
	defer r.deleteCmd(id)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command execution failed: %w", err)
	}

	return nil
}

func (r *Runner) Plan(timestamp int64, params ...string) ([]byte, error) {
	args := []string{"plan"}
	out := []byte{}

	if len(params) > 0 {
		args = append(args, params...)
	}

	args = append(args, "-no-color", "-out", "plan/terraform.plan")

	cmd, id := r.newCmd(args)
	defer r.deleteCmd(id)

	if err := cmd.Run(); err != nil {
		return out, fmt.Errorf("command execution failed: %w", err)
	}

	err := os.WriteFile(path.Join(r.paths.Plan,
		fmt.Sprintf("plan-%d.log", timestamp)),
		cmd.Log.Out.Bytes(),
		iox.FullRWPermAccess)
	if err != nil {
		return out, fmt.Errorf("error writing terraform plan log: %w", err)
	}

	out = cmd.Log.Out.Bytes()

	return out, nil
}

func (r *Runner) Apply(timestamp int64) (OutputJSON, error) {
	var oj OutputJSON

	cmd, applyID := r.newCmd([]string{"apply", "-no-color", "-json", "plan/terraform.plan"})
	defer r.deleteCmd(applyID)

	if err := cmd.Run(); err != nil {
		return oj, fmt.Errorf("cannot create cloud resources: %w", err)
	}

	if err := os.WriteFile(
		path.Join(r.paths.Logs, fmt.Sprintf("%d.log", timestamp)),
		cmd.Log.Out.Bytes(),
		iox.FullRWPermAccess,
	); err != nil {
		return oj, fmt.Errorf("error writing terraform apply log: %w", err)
	}

	cmd, outputID := r.newCmd([]string{"output", "-json"})
	defer r.deleteCmd(outputID)

	if err := cmd.Run(); err != nil {
		return oj, fmt.Errorf("cannot access terraform apply output: %w", err)
	}

	if err := os.WriteFile(
		path.Join(r.paths.Outputs, "output.json"),
		cmd.Log.Out.Bytes(),
		iox.FullRWPermAccess,
	); err != nil {
		return oj, fmt.Errorf("error writing terraform apply outputs: %w", err)
	}

	if err := json.Unmarshal(cmd.Log.Out.Bytes(), &oj); err != nil {
		return oj, fmt.Errorf("error unmarshalling terraform apply outputs: %w", err)
	}

	return oj, nil
}

func (r *Runner) Destroy() error {
	args := []string{"destroy", "-auto-approve"}

	if execx.NoTTY {
		args = append(args, "-no-color")
	}

	cmd, id := r.newCmd(args)
	defer r.deleteCmd(id)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error running terraform destroy: %w", err)
	}

	return nil
}

func (r *Runner) Version() (string, error) {
	args := []string{"version"}

	cmd, id := r.newCmd(args)
	defer r.deleteCmd(id)

	log, err := execx.CombinedOutput(cmd)
	if err != nil {
		return "", fmt.Errorf("error running terraform version: %w", err)
	}

	return log, nil
}

func (r *Runner) Stop() error {
	for _, cmd := range r.cmds {
		if err := cmd.Stop(); err != nil {
			return fmt.Errorf("error stopping terraform runner: %w", err)
		}
	}

	return nil
}
