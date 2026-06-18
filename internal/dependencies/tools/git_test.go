// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package tools_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/dependencies/tools"
	"github.com/sighupio/furyctl/internal/tool/git"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

func Test_Git_CheckBinVersion(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc                string
		testHelperProcessFn string
		wantVersion         string
		wantErr             bool
		wantErrMsg          string
	}{
		{
			desc:                "correct version installed",
			testHelperProcessFn: "TestHelperProcessGitStandard",
			wantVersion:         "2.39.0",
		},
		{
			desc:                "correct version installed, apple flavor",
			testHelperProcessFn: "TestHelperProcessGitApple",
			wantVersion:         "2.37.0",
		},
		{
			desc:                "wrong version installed",
			testHelperProcessFn: "TestHelperProcessGitStandard",
			wantVersion:         "2.39.1",
			wantErr:             true,
			wantErrMsg:          "installed = 2.39.0, expected = 2.39.1",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			fa := tools.NewGit(newGitRunner(tC.testHelperProcessFn), tC.wantVersion)

			err := fa.CheckBinVersion()

			if tC.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tC.wantErrMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func newGitRunner(testHelperProcessFn string) *git.Runner {
	return git.NewRunner(execx.NewFakeExecutor(testHelperProcessFn), git.Paths{
		Git: "git",
	})
}

func TestHelperProcessGitStandard(t *testing.T) {
	args := os.Args

	if len(args) < 3 || args[1] != "-test.run=TestHelperProcessGitStandard" {
		return
	}

	cmd, subcmd := args[3], args[4]

	switch cmd {
	case "git":
		switch subcmd {
		case "version":
			fmt.Fprintf(os.Stdout, "git version 2.39.0\n")
		}
	default:
		fmt.Fprintf(os.Stdout, "command not found")
	}

	os.Exit(0)
}

func TestHelperProcessGitApple(t *testing.T) {
	args := os.Args

	if len(args) < 3 || args[1] != "-test.run=TestHelperProcessGitApple" {
		return
	}

	cmd, subcmd := args[3], args[4]

	switch cmd {
	case "git":
		switch subcmd {
		case "version":
			fmt.Fprintf(os.Stdout, "git version 2.37.0 (Apple Git-136)\n")
		}
	default:
		fmt.Fprintf(os.Stdout, "command not found")
	}

	os.Exit(0)
}
