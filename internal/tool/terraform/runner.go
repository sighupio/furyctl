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

var errOutputFromApply = errors.New("can't get outputs from terraform apply logs")

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
}

func NewRunner(executor execx.Executor, paths Paths) *Runner {
	return &Runner{
		executor: executor,
		paths:    paths,
	}
}

func (r *Runner) Init() error {
	err := execx.NewCmd(r.paths.Terraform, execx.CmdOptions{
		Args:     []string{"init"},
		Executor: r.executor,
		WorkDir:  r.paths.WorkDir,
	}).Run()
	if err != nil {
		return fmt.Errorf("error running terraform init: %w", err)
	}

	return nil
}

func (r *Runner) Plan(timestamp int64) error {
	cmd := execx.NewCmd(r.paths.Terraform, execx.CmdOptions{
		Args:     []string{"plan", "--out=plan/terraform.plan", "-no-color"},
		Executor: r.executor,
		WorkDir:  r.paths.WorkDir,
	})
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error running terraform plan: %w", err)
	}

	err := os.WriteFile(path.Join(r.paths.Plan,
		fmt.Sprintf("plan-%d.log", timestamp)),
		cmd.Log.Out.Bytes(),
		iox.FullRWPermAccess)
	if err != nil {
		return fmt.Errorf("error writing terraform plan log: %w", err)
	}

	return nil
}

func (r *Runner) Apply(timestamp int64) (OutputJSON, error) {
	var oj OutputJSON

	cmd := execx.NewCmd(r.paths.Terraform, execx.CmdOptions{
		Args:     []string{"apply", "-no-color", "-json", "plan/terraform.plan"},
		Executor: r.executor,
		WorkDir:  r.paths.WorkDir,
	})
	if err := cmd.Run(); err != nil {
		return oj, fmt.Errorf("error running terraform apply: %w", err)
	}

	err := os.WriteFile(path.Join(r.paths.Logs,
		fmt.Sprintf("%d.log", timestamp)),
		cmd.Log.Out.Bytes(),
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

	return oj, nil
}

func (r *Runner) Version() (string, error) {
	log, err := execx.CombinedOutput(execx.NewCmd(r.paths.Terraform, execx.CmdOptions{
		Args:     []string{"version"},
		Executor: r.executor,
		WorkDir:  r.paths.WorkDir,
	}))
	if err != nil {
		return "", fmt.Errorf("error running terraform version: %w", err)
	}

	return log, nil
}
