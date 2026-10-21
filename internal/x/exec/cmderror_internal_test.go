// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package execx

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTail(t *testing.T) {
	t.Parallel()

	lines := []string{"", "a", "", "b", "c", "", "d", ""}

	assert.Equal(t, []string{"c", "", "d"}, Tail(lines, 2))
	assert.Equal(t, []string{"a", "", "b", "c", "", "d"}, Tail(lines, 10))
	assert.Empty(t, Tail([]string{"", ""}, 10))
}

func TestCmdError_Error(t *testing.T) {
	t.Parallel()

	e := &CmdError{
		Err:     errors.New("exit status 2"),
		Summary: "it failed on 1 host",
		LeadIn:  "Recap:",
		Block:   []string{"PLAY RECAP   ", "", " host : ok=1"},
		Note:    "(1 warning not shown)",
		command: "tool run",
		file:    "logs/tool-run.log",
	}

	want := "it failed on 1 host\n\ncommand failed: tool run (exit status 2)\n\nRecap:\n" +
		" │ PLAY RECAP\n │\n │  host : ok=1\n(1 warning not shown)\n\n" +
		"The full output of the command is in logs/tool-run.log"
	assert.Equal(t, want, e.Error())
}

func TestSaveOutput(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "logs")
	now := time.Date(2026, 10, 15, 18, 53, 39, 121e6, time.UTC)

	file, err := saveOutput(dir, "sh-my apply.sh", now, []string{"one", "", "two"})
	require.NoError(t, err)

	assert.Equal(t, filepath.Join(dir, "sh-my_apply.sh-20261015-185339.121.log"), file)

	got, err := os.ReadFile(file)
	require.NoError(t, err)
	assert.Equal(t, "one\n\ntwo\n", string(got))

	info, err := os.Stat(file)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	_, err = saveOutput("", "x", now, nil)
	require.ErrorIs(t, err, ErrNoLogDir)
}

func TestLineLog(t *testing.T) {
	t.Parallel()

	l := &lineLog{}
	out, errOut := lineWriter{log: l}, lineWriter{log: l, stderr: true}

	_, _ = out.Write([]byte("a\nb"))
	_, _ = errOut.Write([]byte("\x1b[31mw\x1b[0m\n"))
	_, _ = out.Write([]byte("c\r\nd"))

	assert.Equal(t, []Line{
		{Text: "a"},
		{Text: "w", Stderr: true},
		{Text: "bc"},
		{Text: "d"},
	}, l.all())
}

// Not parallel: it sets the shared LogDir.
func TestCmd_Run_OutputFile(t *testing.T) {
	saved := LogDir
	LogDir = t.TempDir()

	defer func() { LogDir = saved }()

	script := filepath.Join(t.TempDir(), "fail.sh")
	require.NoError(t, os.WriteFile(script, []byte("echo out\nsleep 0.1\necho err >&2\nexit 1\n"), 0o600))

	err := NewCmd("sh", CmdOptions{Args: []string{script}}).Run()
	require.ErrorIs(t, err, ErrCmdFailed)

	files, err2 := filepath.Glob(filepath.Join(LogDir, "sh-fail.sh-*.log"))
	require.NoError(t, err2)
	require.Len(t, files, 1, "the file is named after the program and its first argument")

	info, err2 := os.Stat(files[0])
	require.NoError(t, err2)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	data, err2 := os.ReadFile(files[0])
	require.NoError(t, err2)
	assert.Equal(t, "out\nerr\n", string(data), "stdout and stderr in arrival order")

	assert.Contains(t, err.Error(), "The full output of the command is in "+files[0])
}

// Not parallel: it sets the shared LogDir.
func TestCmd_Run_NoOutputFile(t *testing.T) {
	saved := LogDir

	defer func() { LogDir = saved }()

	for name, cmd := range map[string]func() *Cmd{
		"probe": func() *Cmd {
			c := NewCmd("sh", CmdOptions{Args: []string{"-c", "echo no state; exit 1"}})
			c.Probe = true

			return c
		},
		"no output": func() *Cmd {
			return NewCmd("sh", CmdOptions{Args: []string{"-c", "exit 1"}})
		},
		"sensitive": func() *Cmd {
			return NewCmd("sh", CmdOptions{Args: []string{"-c", "echo secret; exit 1"}, Sensitive: true})
		},
	} {
		LogDir = t.TempDir()

		err := cmd().Run()
		require.ErrorIs(t, err, ErrCmdFailed, name)

		files, err2 := os.ReadDir(LogDir)
		require.NoError(t, err2, name)
		assert.Empty(t, files, name)
		assert.NotContains(t, err.Error(), "The full output of the command is in", name)
	}
}

// Not parallel: it sets the shared LogDir.
func TestRunWithExitCode_NonZeroExitSavesNoOutputFile(t *testing.T) {
	saved := LogDir

	defer func() { LogDir = saved }()

	LogDir = t.TempDir()

	stdout, stderr, code, err := RunWithExitCode(
		NewCmd("sh", CmdOptions{Args: []string{"-c", "echo plan; echo boom >&2; exit 3"}}),
	)
	require.NoError(t, err)
	assert.Equal(t, "plan\n", stdout)
	assert.Equal(t, "boom\n", stderr)
	assert.Equal(t, 3, code)

	files, err := os.ReadDir(LogDir)
	require.NoError(t, err)
	assert.Empty(t, files)
}

func TestStderrBlock(t *testing.T) {
	t.Parallel()

	// kubectl version -o json: the error on stderr first, then the client version on stdout.
	e := &CmdError{
		Lines: []Line{
			{Text: "Unable to connect to the server: no route to host", Stderr: true},
			{Text: "{"},
			{Text: `  "clientVersion": {}`},
			{Text: "}"},
		},
		Block: []string{"{", `  "clientVersion": {}`, "}"},
	}
	StderrBlock(e)
	assert.Equal(t, []string{"Unable to connect to the server: no route to host"}, e.Block)

	// Nothing on stderr: the block stays as it is.
	e = &CmdError{Lines: []Line{{Text: "out"}}, Block: []string{"out"}}
	StderrBlock(e)
	assert.Equal(t, []string{"out"}, e.Block)
}
