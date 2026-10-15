// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package shell_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/tool/shell"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

func newScriptRunner(t *testing.T, script string) (*shell.Runner, string, string) {
	t.Helper()

	workDir := t.TempDir()
	scriptPath := filepath.Join(t.TempDir(), "script.sh")

	require.NoError(t, os.WriteFile(scriptPath, []byte(script), os.FileMode(0o600)))

	r := shell.NewRunner(execx.NewStdExecutor(), shell.Paths{Shell: "sh", WorkDir: workDir})

	return r, scriptPath, workDir
}

func TestRunCapture_OutputAndExitCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		script string
		out    string
		err    string
		code   int
	}{
		{"failing script", "echo kapp-out\necho kapp-err >&2\nexit 3\n", "kapp-out", "kapp-err", 3},
		{"succeeding script", "echo diff-started\necho diff-warning >&2\n", "diff-started", "diff-warning", 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r, script, _ := newScriptRunner(t, tc.script)

			out, code, err := r.RunCapture(script)

			require.NoError(t, err)
			require.Equal(t, tc.code, code)
			require.Contains(t, out, tc.out)
			require.Contains(t, out, tc.err)
		})
	}
}

func TestRunCapture_AddsNoArgument(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want string
	}{
		{"none", nil, ""},
		{"two", []string{"alpha", "--beta"}, "alpha\n--beta"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r, script, _ := newScriptRunner(t, "for a in \"$@\"; do echo \"$a\"; done\n")

			out, code, err := r.RunCapture(append([]string{script}, tc.args...)...)

			require.NoError(t, err)
			require.Equal(t, 0, code)
			require.Equal(t, tc.want, strings.TrimSpace(out))
		})
	}
}

func TestRunCapture_InheritsEnvironment(t *testing.T) {
	t.Setenv("FURYCTL_RUN_CAPTURE_PROBE", "probe-value")

	r, script, _ := newScriptRunner(t, "env\n")

	out, code, err := r.RunCapture(script)

	require.NoError(t, err)
	require.Equal(t, 0, code)

	got := map[string]bool{}
	for _, l := range strings.Split(out, "\n") {
		got[l] = true
	}

	require.True(t, got["FURYCTL_RUN_CAPTURE_PROBE=probe-value"])

	// The child sees the process environment, apart from the variables sh itself sets.
	for _, e := range os.Environ() {
		if strings.ContainsAny(e, "\n") || strings.HasPrefix(e, "_=") || strings.HasPrefix(e, "SHLVL=") ||
			strings.HasPrefix(e, "PWD=") || strings.HasPrefix(e, "OLDPWD=") {
			continue
		}

		require.True(t, got[e], "missing %s", strings.SplitN(e, "=", 2)[0])
	}
}

func TestRunCapture_RunsInWorkDir(t *testing.T) {
	t.Parallel()

	r, script, workDir := newScriptRunner(t, "pwd -P\n")

	out, _, err := r.RunCapture(script)

	require.NoError(t, err)

	want, evalErr := filepath.EvalSymlinks(workDir)
	require.NoError(t, evalErr)
	require.Equal(t, want, strings.TrimSpace(out))
}
