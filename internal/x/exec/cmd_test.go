// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package execx_test

import (
	"bytes"
	"errors"
	"os"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	execx "github.com/sighupio/furyctl/internal/x/exec"
)

var ErrTest = errors.New("test error")

func Test_Cmd_Run_Error(t *testing.T) {
	t.Parallel()

	err := execx.NewCmd("sh", execx.CmdOptions{
		Args: []string{"-c", "echo out; printf '\\n\\n'; sleep 0.1; echo err >&2; exit 3"},
	}).Run()
	require.ErrorIs(t, err, execx.ErrCmdFailed)

	want := "command failed: sh -c echo out; printf '\\n\\n'; sleep 0.1; echo err >&2; exit 3 (exit status 3)\n\n" +
		"Last lines of the command output:\n │ out\n │\n │\n │ err"
	assert.Equal(t, want, err.Error())
}

func Test_Cmd_Run_OnError(t *testing.T) {
	t.Parallel()

	cmd := execx.NewCmd("sh", execx.CmdOptions{Args: []string{"-c", "echo out; exit 1"}})
	cmd.OnError = func(e *execx.CmdError) {
		e.Summary = "it broke"
		e.Name = "tool"
		e.Args = e.Args[1:]
		e.Block = nil
	}

	err := cmd.Run()
	require.ErrorIs(t, err, execx.ErrCmdFailed)

	assert.Equal(t, "it broke\n\ncommand failed: tool echo out; exit 1 (exit status 1)", err.Error())
}

func TestNewCmd(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc        string
		opts        execx.CmdOptions
		wantArgs    []string
		wantWorkDir bool
	}{
		{
			desc: "empty options",
		},
		{
			desc: "full options",
			opts: execx.CmdOptions{
				Args:     []string{"foo", "bar"},
				Err:      bytes.NewBufferString("bar"),
				Executor: execx.NewFakeExecutor("TestHelperProcess"),
				Out:      bytes.NewBufferString("foo"),
				WorkDir:  os.TempDir(),
			},
			wantArgs:    []string{"foo", "bar"},
			wantWorkDir: true,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			cmd := execx.NewCmd("echo", tC.opts)

			if cmd == nil {
				t.Fatal("cmd is nil")
			}

			if cmd.Path == "" {
				t.Errorf("cmd.Path is empty")
			}

			for _, wantArg := range tC.wantArgs {
				if !slices.Contains(cmd.Args, wantArg) {
					t.Errorf("cmd.Args should contain %v but it does not: %v", wantArg, cmd.Args)
				}
			}

			if cmd.Log == nil {
				t.Error("cmd.Log is nil")
			}

			if (tC.wantWorkDir != false) && cmd.Dir == "" {
				t.Errorf("cmd.Dir = %s, got = %s", "", cmd.Dir)
			}
		})
	}
}

func Test_Cmd_Run(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc    string
		cmd     *execx.Cmd
		wantErr bool
	}{
		{
			desc:    "failed run",
			cmd:     execx.NewCmd("false", execx.CmdOptions{}),
			wantErr: true,
		},
		{
			desc:    "successful run",
			cmd:     execx.NewCmd("true", execx.CmdOptions{}),
			wantErr: false,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			err := tC.cmd.Run()
			if (err != nil) != tC.wantErr {
				t.Errorf("Cmd.Run() error = %v, wantErr = %v", err, tC.wantErr)
			}

			if tC.wantErr && !errors.Is(err, execx.ErrCmdFailed) {
				t.Errorf("Cmd.Err = %v, want = %v", tC.cmd.Err, execx.ErrCmdFailed)
			}
		})
	}
}

func Test_Cmd_Stop(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc    string
		cmd     *execx.Cmd
		wantErr bool
	}{
		{
			desc: "successful stop",
			cmd: execx.NewCmd("long process", execx.CmdOptions{
				Args:     []string{"sleep", "60"},
				Executor: execx.NewFakeExecutor("TestHelperProcess"),
			}),
			wantErr: false,
		},
	}

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			err := tC.cmd.Stop()
			if (err != nil) != tC.wantErr {
				t.Errorf("Cmd.Stop() error = %v, wantErr = %v", err, tC.wantErr)
			}

			if tC.wantErr && !errors.Is(err, execx.ErrCmdFailed) {
				t.Errorf("Cmd.Err = %v, want = %v", tC.cmd.Err, execx.ErrCmdFailed)
			}
		})
	}
}

func TestCombinedOutput(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc  string
		want  string
		setup func() *execx.Cmd
	}{
		{
			desc: "no out, no err",
			want: "",
			setup: func() *execx.Cmd {
				return execx.NewCmd("echo", execx.CmdOptions{})
			},
		},
		{
			desc: "out, no err",
			want: "foo",
			setup: func() *execx.Cmd {
				return execx.NewCmd("echo", execx.CmdOptions{
					Args: []string{"foo"},
				})
			},
		},
		{
			desc: "no out, err",
			want: "bar",
			setup: func() *execx.Cmd {
				cmd := execx.NewCmd("echo", execx.CmdOptions{})

				cmd.Log.Err.WriteString("bar")

				return cmd
			},
		},
		{
			desc: "out, err",
			want: "foo\nbar",
			setup: func() *execx.Cmd {
				cmd := execx.NewCmd("echo", execx.CmdOptions{
					Args: []string{"foo"},
				})

				cmd.Log.Err.WriteString("bar")

				return cmd
			},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			ret, err := execx.CombinedOutput(tC.setup())
			if err != nil {
				t.Fatal(err)
			}

			if ret != tC.want {
				t.Errorf("want = %s, got = %s", tC.want, ret)
			}
		})
	}
}
