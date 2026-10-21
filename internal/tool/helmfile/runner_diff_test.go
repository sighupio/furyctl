// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package helmfile_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/tool/helmfile"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

func TestRunnerDiff_CommandLine(t *testing.T) {
	t.Setenv("FAKE_HELMFILE_STDOUT", "stdout")

	workDir := t.TempDir()

	r := helmfile.NewRunner(execx.NewFakeExecutor("TestDiffHelperProcess"), helmfile.Paths{
		Helmfile: "helmfile",
		WorkDir:  workDir,
	})

	stdout, _, _, err := r.Diff()

	require.NoError(t, err)

	want, evalErr := filepath.EvalSymlinks(workDir)
	require.NoError(t, evalErr)

	require.Contains(t, stdout, "ARGS=helmfile diff --output simple --no-color --suppress-secrets\n")
	require.Contains(t, stdout, "UPGRADE_NOTICE=1\n")
	require.Contains(t, stdout, "DIR="+want+"\n")
}

func TestRunnerDiff_StreamsAndExitCode(t *testing.T) {
	tests := []struct {
		name   string
		stdout string
		stderr string
		code   int
	}{
		{"failing", "Comparing release=poc-podinfo, chart=podinfo/podinfo", "Error: Failed to get release", 1},
		{"succeeding", "Plan: 0 to add, 0 to change, 0 to destroy.", `"podinfo" has been added`, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("FAKE_HELMFILE_STDOUT", tc.stdout)
			t.Setenv("FAKE_HELMFILE_STDERR", tc.stderr)
			t.Setenv("FAKE_HELMFILE_CODE", strconv.Itoa(tc.code))

			r := helmfile.NewRunner(execx.NewFakeExecutor("TestDiffHelperProcess"), helmfile.Paths{
				Helmfile: "helmfile",
				WorkDir:  t.TempDir(),
			})

			stdout, stderr, code, err := r.Diff()

			require.NoError(t, err)
			require.Equal(t, tc.code, code)
			require.True(t, strings.HasPrefix(stdout, tc.stdout+"\n"))
			require.Equal(t, tc.stderr+"\n", stderr)
		})
	}
}

func TestDiffHelperProcess(t *testing.T) {
	if len(os.Args) < 3 || os.Args[1] != "-test.run=TestDiffHelperProcess" {
		return
	}

	dir, _ := os.Getwd()

	fmt.Fprintln(os.Stdout, os.Getenv("FAKE_HELMFILE_STDOUT"))
	fmt.Fprintf(os.Stdout, "ARGS=%s\n", strings.Join(os.Args[3:], " "))
	fmt.Fprintf(os.Stdout, "UPGRADE_NOTICE=%s\n", os.Getenv("HELMFILE_UPGRADE_NOTICE_DISABLED"))
	fmt.Fprintf(os.Stdout, "DIR=%s\n", dir)
	fmt.Fprintln(os.Stderr, os.Getenv("FAKE_HELMFILE_STDERR"))

	code, _ := strconv.Atoi(os.Getenv("FAKE_HELMFILE_CODE"))
	os.Exit(code)
}
