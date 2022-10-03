// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package terraform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"regexp"

	tfjson "github.com/hashicorp/terraform-json"
)

type OutputJson struct {
	Outputs map[string]*tfjson.StateOutput `json:"outputs"`
}

type Paths struct {
	Logs      string
	Outputs   string
	Base      string
	Plan      string
	Terraform string
}

type Runner struct {
	paths Paths
}

func NewRunner(paths Paths) *Runner {
	return &Runner{
		paths: paths,
	}
}

func (r *Runner) newCmd(stdOut io.Writer, args ...string) *exec.Cmd {
	cmd := exec.Command(r.paths.Terraform, args...)
	cmd.Stdout = stdOut
	cmd.Stderr = os.Stderr
	cmd.Dir = path.Join(r.paths.Base, "terraform")

	return cmd
}

func (r *Runner) Init() error {
	return r.newCmd(os.Stdout, "init").Run()
}

func (r *Runner) Plan(timestamp int64) error {
	var planBuffer bytes.Buffer

	cmd := r.newCmd(io.MultiWriter(os.Stdout, &planBuffer), "plan", "--out=plan/terraform.plan", "-no-color")
	if err := cmd.Run(); err != nil {
		return err
	}

	logFilePath := fmt.Sprintf("plan-%d.log", timestamp)

	return os.WriteFile(path.Join(r.paths.Plan, logFilePath), planBuffer.Bytes(), 0o600)
}

func (r *Runner) Apply(timestamp int64) (OutputJson, error) {
	var applyBuffer bytes.Buffer
	var applyLogOut OutputJson

	cmd := r.newCmd(io.MultiWriter(os.Stdout, &applyBuffer), "apply", "-no-color", "-json", "plan/terraform.plan")
	if err := cmd.Run(); err != nil {
		return applyLogOut, err
	}

	err := os.WriteFile(path.Join(r.paths.Logs, fmt.Sprintf("%d.log", timestamp)), applyBuffer.Bytes(), 0o600)
	if err != nil {
		return applyLogOut, err
	}

	parsedApplyLog, err := os.ReadFile(path.Join(r.paths.Logs, fmt.Sprintf("%d.log", timestamp)))
	if err != nil {
		return applyLogOut, err
	}

	applyLog := string(parsedApplyLog)

	pattern := regexp.MustCompile("(\"outputs\":){(.*?)}}")

	outputsStringIndex := pattern.FindStringIndex(applyLog)
	if outputsStringIndex == nil {
		return applyLogOut, fmt.Errorf("can't get outputs from terraform apply logs")
	}

	outputsString := fmt.Sprintf("{%s}", applyLog[outputsStringIndex[0]:outputsStringIndex[1]])

	if err := json.Unmarshal([]byte(outputsString), &applyLogOut); err != nil {
		return applyLogOut, err
	}

	return applyLogOut, os.WriteFile(path.Join(r.paths.Outputs, "output.json"), []byte(outputsString), 0o600)
}
