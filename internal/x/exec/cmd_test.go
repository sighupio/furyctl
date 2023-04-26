// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package execx_test

import (
	"bytes"
	"errors"
	"os"
	"testing"

	"golang.org/x/exp/slices"

	execx "github.com/sighupio/furyctl/internal/x/exec"
)

func TestNewErrCmdFailed(t *testing.T) {
	err := execx.NewErrCmdFailed("foo", []string{"bar", "baz"}, errors.New("test error"), nil)

	if err == nil {
		t.Error("error is nil")
	}

	wantErr := "foo bar baz: command failed - test error\n<nil>"
	if err.Error() != wantErr {
		t.Errorf("wantErr = %s, got = %s", wantErr, err.Error())
	}
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
				Executor: execx.NewFakeExecutor(),
				Out:      bytes.NewBufferString("foo"),
				WorkDir:  os.TempDir(),
			},
			wantArgs:    []string{"foo", "bar"},
			wantWorkDir: true,
		},
	}
	for _, tC := range testCases {
		tC := tC

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
			desc:    "succesful run",
			cmd:     execx.NewCmd("true", execx.CmdOptions{}),
			wantErr: false,
		},
	}
	for _, tC := range testCases {
		tC := tC

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
			desc: "succesful stop",
			cmd: execx.NewCmd("long process", execx.CmdOptions{
				Args:     []string{"sleep", "60"},
				Executor: execx.NewFakeExecutor(),
			}),
			wantErr: false,
		},
	}
	for _, tC := range testCases {
		tC := tC

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

func Test_CmdLog_String(t *testing.T) {
	cmdLog := &execx.CmdLog{
		Out: bytes.NewBufferString("foo"),
		Err: bytes.NewBufferString("bar"),
	}

	want := "out: foo\nerr: bar\n"
	if cmdLog.String() != want {
		t.Errorf("want = %s, got = %s", want, cmdLog.String())
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
		tC := tC

		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			ret, err := execx.CombinedOutput(tC.setup())
			if err != nil {
				t.Fatal(err)
			}

			if string(ret) != tC.want {
				t.Errorf("want = %s, got = %s", tC.want, string(ret))
			}
		})
	}
}
