// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package terraform_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/sighupio/furyctl/internal/test"
	"github.com/sighupio/furyctl/internal/tool/terraform"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

func Test_Runner_Init(t *testing.T) {
	r := terraform.NewRunner(execx.NewFakeExecutor(), terraform.Paths{
		Terraform: "terraform",
		WorkDir:   test.MkdirTemp(t),
	})

	if err := r.Init(); err != nil {
		t.Fatal(err)
	}
}

func Test_Runner_Plan(t *testing.T) {
	paths := terraform.Paths{
		Terraform: "terraform",
		WorkDir:   test.MkdirTemp(t),
		Logs:      test.MkdirTemp(t),
		Outputs:   test.MkdirTemp(t),
		Plan:      test.MkdirTemp(t),
	}

	r := terraform.NewRunner(execx.NewFakeExecutor(), paths)

	if _, err := r.Plan(42); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(filepath.Join(paths.Plan, "plan-42.log"))
	if err != nil {
		t.Fatal(err)
	}

	if info.Size() == 0 {
		t.Error("expected file to be not empty")
	}
}

func Test_Runner_Apply(t *testing.T) {
	paths := terraform.Paths{
		Terraform: "terraform",
		WorkDir:   test.MkdirTemp(t),
		Logs:      test.MkdirTemp(t),
		Outputs:   test.MkdirTemp(t),
		Plan:      test.MkdirTemp(t),
	}

	r := terraform.NewRunner(execx.NewFakeExecutor(), paths)

	if _, err := r.Apply(42); err != nil {
		t.Fatal(err)
	}

	info1, err := os.Stat(filepath.Join(paths.Logs, "42.log"))
	if err != nil {
		t.Fatal(err)
	}

	if info1.Size() == 0 {
		t.Error("expected '42.log' file to be not empty")
	}

	info2, err := os.Stat(filepath.Join(paths.Outputs, "output.json"))
	if err != nil {
		t.Fatal(err)
	}

	if info2.Size() == 0 {
		t.Error("expected 'output.json' file to be not empty")
	}

	out, err := os.ReadFile(filepath.Join(paths.Outputs, "output.json"))
	if err != nil {
		t.Fatal(err)
	}

	want := `{"outputs":{"foo":{"sensitive":false,"value":"bar"}}}`

	if string(out) != want {
		t.Errorf("expected output to be '%s', got '%s'", want, string(out))
	}
}

func Test_Runner_Version(t *testing.T) {
	r := terraform.NewRunner(execx.NewFakeExecutor(), terraform.Paths{
		Terraform: "terraform",
		WorkDir:   test.MkdirTemp(t),
	})

	got, err := r.Version()
	if err != nil {
		t.Fatal(err)
	}

	want := "v1.2.3"

	if got != want {
		t.Errorf("expected version '%s', got '%s'", want, got)
	}
}

func TestHelperProcess(t *testing.T) {
	args := os.Args

	if len(args) < 3 || args[1] != "-test.run=TestHelperProcess" {
		return
	}

	cmd, subcmd := args[3], args[4]

	switch cmd {
	case "terraform":
		switch subcmd {
		case "init":
			fmt.Fprintf(os.Stdout, "initialized")
		case "plan":
			fmt.Fprintf(os.Stdout, "planned")
		case "apply":
			fmt.Fprintf(os.Stdout, `{"outputs":{"foo":{"sensitive":false,"value":"bar"}}}`)
		case "version":
			fmt.Fprintf(os.Stdout, "v1.2.3")
		default:
			fmt.Fprintf(os.Stdout, "subcommand '%s' not found", subcmd)
		}
	default:
		fmt.Fprintf(os.Stdout, "command '%s' not found", cmd)
	}

	os.Exit(0)
}
