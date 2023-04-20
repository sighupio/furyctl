// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package terraform

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"regexp"

	tfjson "github.com/hashicorp/terraform-json"

	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
)

var (
	errOutputFromApply = errors.New("can't get outputs from terraform apply logs")
	errAlreadyRunning  = errors.New("already running")
)

type OutputJSON struct {
	Outputs map[string]*tfjson.StateOutput `json:"outputs"`
}

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
	cmd      *execx.Cmd
}

func NewRunner(executor execx.Executor, paths Paths) *Runner {
	return &Runner{
		executor: executor,
		paths:    paths,
	}
}

func (r *Runner) CmdPath() string {
	return r.paths.Terraform
}

func (r *Runner) Init() error {
	if r.cmd != nil {
		return errAlreadyRunning
	}

	args := []string{"init"}

	if execx.NoTTY {
		args = append(args, "-no-color")
	}

	r.cmd = r.initCmd(args)

	if err := r.cmd.Run(); err != nil {
		return fmt.Errorf("command execution failed: %w", err)
	}

	r.cmd = nil

	return nil
}

func (r *Runner) Plan(timestamp int64, params ...string) error {
	if r.cmd != nil {
		return errAlreadyRunning
	}

	args := []string{"plan"}

	if len(params) > 0 {
		args = append(args, params...)
	}

	args = append(args, "-no-color", "-out", "plan/terraform.plan")

	r.cmd = r.initCmd(args)

	if err := r.cmd.Run(); err != nil {
		return fmt.Errorf("error running terraform plan: %w", err)
	}

	err := os.WriteFile(path.Join(r.paths.Plan,
		fmt.Sprintf("plan-%d.log", timestamp)),
		r.cmd.Log.Out.Bytes(),
		iox.FullRWPermAccess)
	if err != nil {
		return fmt.Errorf("error writing terraform plan log: %w", err)
	}

	r.cmd = nil

	return nil
}

func (r *Runner) Apply(timestamp int64) (OutputJSON, error) {
	var oj OutputJSON

	if r.cmd != nil {
		return oj, errAlreadyRunning
	}

	args := []string{"apply", "-no-color", "-json", "plan/terraform.plan"}

	r.cmd = r.initCmd(args)

	if err := r.cmd.Run(); err != nil {
		return oj, fmt.Errorf("cannot create cloud resources: %w", err)
	}

	err := os.WriteFile(path.Join(r.paths.Logs,
		fmt.Sprintf("%d.log", timestamp)),
		r.cmd.Log.Out.Bytes(),
		iox.FullRWPermAccess)
	if err != nil {
		return oj, fmt.Errorf("error writing terraform apply log: %w", err)
	}

	parsedApplyLog, err := os.ReadFile(path.Join(r.paths.Logs, fmt.Sprintf("%d.log", timestamp)))
	if err != nil {
		return oj, fmt.Errorf("error reading terraform apply log: %w", err)
	}

	applyLog := string(parsedApplyLog)

	pattern := regexp.MustCompile("(\"outputs\":){(.*?)}}")

	outputsStringIndex := pattern.FindStringIndex(applyLog)
	if outputsStringIndex == nil {
		return oj, errOutputFromApply
	}

	outputsString := fmt.Sprintf("{%s}", applyLog[outputsStringIndex[0]:outputsStringIndex[1]])

	if err := json.Unmarshal([]byte(outputsString), &oj); err != nil {
		return oj, fmt.Errorf("error unmarshalling terraform apply outputs: %w", err)
	}

	err = os.WriteFile(path.Join(r.paths.Outputs, "output.json"), []byte(outputsString), iox.FullRWPermAccess)
	if err != nil {
		return oj, fmt.Errorf("error writing terraform apply outputs: %w", err)
	}

	r.cmd = nil

	return oj, nil
}

func (r *Runner) Destroy() error {
	if r.cmd != nil {
		return errAlreadyRunning
	}

	args := []string{"destroy", "-auto-approve"}

	if execx.NoTTY {
		args = append(args, "-no-color")
	}

	r.cmd = r.initCmd(args)

	if err := r.cmd.Run(); err != nil {
		return fmt.Errorf("error running terraform destroy: %w", err)
	}

	r.cmd = nil

	return nil
}

func (r *Runner) Version() (string, error) {
	if r.cmd != nil {
		return "", errAlreadyRunning
	}

	args := []string{"version"}

	r.cmd = r.initCmd(args)

	log, err := execx.CombinedOutput(r.cmd)
	if err != nil {
		return "", fmt.Errorf("error running terraform version: %w", err)
	}

	r.cmd = nil

	return log, nil
}

func (r *Runner) Stop() error {
	if r.cmd == nil {
		return nil
	}

	if err := r.cmd.Stop(); err != nil {
		fmt.Println("Error stopping terraform runner")
		return fmt.Errorf("error stopping terraform runner: %w", err)
	}

	return nil
}

func (r *Runner) initCmd(args []string) *execx.Cmd {
	return execx.NewCmd(r.paths.Terraform, execx.CmdOptions{
		Executor: r.executor,
		WorkDir:  r.paths.WorkDir,
		Args:     args,
	})
}
