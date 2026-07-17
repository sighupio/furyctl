// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package execx_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	execx "github.com/sighupio/furyctl/internal/x/exec"
)

func Test_StdExecutor_Command(t *testing.T) {
	e := execx.NewStdExecutor()

	cmd := e.Command("echo", "hello go world")
	require.NotNil(t, cmd, "expected command to be not nil")

	out, err := cmd.Output()
	require.NoError(t, err, "expected command to be executed without errors")

	assert.Equal(t, "hello go world\n", string(out))
}

func Test_FakeExecutor_Command(t *testing.T) {
	e := execx.NewFakeExecutor("TestHelperProcess")

	cmd := e.Command("fakectl", "hello world")
	require.NotNil(t, cmd, "expected command to be not nil")

	out, err := cmd.Output()
	require.NoError(t, err, "expected command to be executed without errors")

	t.Log(cmd.Args)

	assert.Equal(t, "hello world", string(out))
}

func TestHelperProcess(t *testing.T) {
	args := os.Args

	if len(args) < 3 || args[1] != "-test.run=TestHelperProcess" {
		return
	}

	cmd, _ := args[3], args[4:]

	switch cmd {
	case "fakectl":
		fmt.Fprintf(os.Stdout, "hello world")
	default:
		fmt.Fprintf(os.Stdout, "command not found")
	}

	os.Exit(0)
}
