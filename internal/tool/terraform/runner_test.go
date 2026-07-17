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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/test"
	"github.com/sighupio/furyctl/internal/tool/terraform"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

func Test_Runner_Init(t *testing.T) {
	r := terraform.NewRunner(execx.NewFakeExecutor("TestHelperProcess"), terraform.Paths{
		Terraform: "terraform",
		WorkDir:   test.MkdirTemp(t),
	})

	err := r.Init()
	require.NoError(t, err)
}

func Test_Runner_Plan(t *testing.T) {
	paths := terraform.Paths{
		Terraform: "terraform",
		WorkDir:   test.MkdirTemp(t),
		Logs:      test.MkdirTemp(t),
		Outputs:   test.MkdirTemp(t),
		Plan:      test.MkdirTemp(t),
	}

	r := terraform.NewRunner(execx.NewFakeExecutor("TestHelperProcess"), paths)

	_, err := r.Plan(42)
	require.NoError(t, err)

	info, err := os.Stat(filepath.Join(paths.Plan, "plan-42.log"))
	require.NoError(t, err)
	assert.NotZero(t, info.Size(), "expected file to be not empty")
}

func Test_Runner_Apply(t *testing.T) {
	paths := terraform.Paths{
		Terraform: "terraform",
		WorkDir:   test.MkdirTemp(t),
		Logs:      test.MkdirTemp(t),
		Outputs:   test.MkdirTemp(t),
		Plan:      test.MkdirTemp(t),
	}

	r := terraform.NewRunner(execx.NewFakeExecutor("TestHelperProcess"), paths)

	err := r.Apply(42)
	require.NoError(t, err)

	info1, err := os.Stat(filepath.Join(paths.Logs, "42.log"))
	require.NoError(t, err)
	assert.NotZero(t, info1.Size(), "expected '42.log' file to be not empty")
}

func Test_Runner_Output(t *testing.T) {
	paths := terraform.Paths{
		Terraform: "terraform",
		WorkDir:   test.MkdirTemp(t),
		Logs:      test.MkdirTemp(t),
		Outputs:   test.MkdirTemp(t),
		Plan:      test.MkdirTemp(t),
	}

	r := terraform.NewRunner(execx.NewFakeExecutor("TestHelperProcess"), paths)

	_, err := r.Output()
	require.NoError(t, err)

	info2, err := os.Stat(filepath.Join(paths.Outputs, "output.json"))
	require.NoError(t, err)
	assert.NotZero(t, info2.Size(), "expected 'output.json' file to be not empty")

	out, err := os.ReadFile(filepath.Join(paths.Outputs, "output.json"))
	require.NoError(t, err)

	assert.Equal(t, `{"outputs":{"foo":{"sensitive":false,"value":"bar"}}}`, string(out))
}

func Test_Runner_Version(t *testing.T) {
	r := terraform.NewRunner(execx.NewFakeExecutor("TestHelperProcess"), terraform.Paths{
		Terraform: "terraform",
		WorkDir:   test.MkdirTemp(t),
	})

	got, err := r.Version()
	require.NoError(t, err)

	assert.Equal(t, "v1.2.3", got)
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
		case "output":
			fmt.Fprintf(os.Stdout, `{"outputs":{"foo":{"sensitive":false,"value":"bar"}}}`)
		default:
			fmt.Fprintf(os.Stdout, "subcommand '%s' not found", subcmd)
		}
	default:
		fmt.Fprintf(os.Stdout, "command '%s' not found", cmd)
	}

	os.Exit(0)
}
