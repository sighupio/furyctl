// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package terraform

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"regexp"

	tfjson "github.com/hashicorp/terraform-json"

	execx "github.com/sighupio/furyctl/internal/x/exec"
)

type OutputJson struct {
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
	return execx.NewCmd(r.paths.Terraform, execx.CmdOptions{
		Args:     []string{"init"},
		Executor: r.executor,
		WorkDir:  r.paths.WorkDir,
	}).Run()
}

func (r *Runner) Plan(timestamp int64) error {
	cmd := execx.NewCmd(r.paths.Terraform, execx.CmdOptions{
		Args:     []string{"plan", "--out=plan/terraform.plan", "-no-color"},
		Executor: r.executor,
		WorkDir:  r.paths.WorkDir,
	})
	if err := cmd.Run(); err != nil {
		return err
	}

	return os.WriteFile(path.Join(r.paths.Plan, fmt.Sprintf("plan-%d.log", timestamp)), cmd.Log.Out.Bytes(), 0o600)
}

func (r *Runner) Apply(timestamp int64) (OutputJson, error) {
	var oj OutputJson

	cmd := execx.NewCmd(r.paths.Terraform, execx.CmdOptions{
		Args:     []string{"apply", "-no-color", "-json", "plan/terraform.plan"},
		Executor: r.executor,
		WorkDir:  r.paths.WorkDir,
	})
	if err := cmd.Run(); err != nil {
		return oj, err
	}

	err := os.WriteFile(path.Join(r.paths.Logs, fmt.Sprintf("%d.log", timestamp)), cmd.Log.Out.Bytes(), 0o600)
	if err != nil {
		return oj, err
	}

	parsedApplyLog, err := os.ReadFile(path.Join(r.paths.Logs, fmt.Sprintf("%d.log", timestamp)))
	if err != nil {
		return oj, err
	}

	applyLog := string(parsedApplyLog)

	pattern := regexp.MustCompile("(\"outputs\":){(.*?)}}")

	outputsStringIndex := pattern.FindStringIndex(applyLog)
	if outputsStringIndex == nil {
		return oj, fmt.Errorf("can't get outputs from terraform apply logs")
	}

	outputsString := fmt.Sprintf("{%s}", applyLog[outputsStringIndex[0]:outputsStringIndex[1]])

	if err := json.Unmarshal([]byte(outputsString), &oj); err != nil {
		return oj, err
	}

	return oj, os.WriteFile(path.Join(r.paths.Outputs, "output.json"), []byte(outputsString), 0o600)
}

func (r *Runner) Version() (string, error) {
	return execx.CombinedOutput(execx.NewCmd(r.paths.Terraform, execx.CmdOptions{
		Args:     []string{"version"},
		Executor: r.executor,
		WorkDir:  r.paths.WorkDir,
	}))
}
