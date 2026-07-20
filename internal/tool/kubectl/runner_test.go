// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package kubectl_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/tool/kubectl"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

func Test_Runner_Version(t *testing.T) {
	r := kubectl.NewRunner(execx.NewFakeExecutor("TestHelperProcess"), kubectl.Paths{
		Kubectl: "kubectl",
		WorkDir: os.TempDir(),
	}, true, true, true)

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
	case "kubectl":
		switch subcmd {
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
