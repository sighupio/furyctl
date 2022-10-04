// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package execx

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

type Executor interface {
	Command(name string, arg ...string) *exec.Cmd
}

func NewStdExecutor() *StdExecutor {
	return &StdExecutor{}
}

type StdExecutor struct{}

func (e *StdExecutor) Command(name string, arg ...string) *exec.Cmd {
	return exec.Command(name, arg...)
}

func NewFakeExecutor() *FakeExecutor {
	return &FakeExecutor{}
}

type FakeExecutor struct{}

func (e *FakeExecutor) Command(name string, arg ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", filepath.Base(name)}
	cs = append(cs, arg...)

	return exec.Command(os.Args[0], cs...)
}

type CmdOptions struct {
	Out      io.Writer
	Err      io.Writer
	Executor Executor
	WorkDir  string
	Args     []string
}

func NewCmd(name string, opts CmdOptions) *exec.Cmd {
	if opts.Executor == nil {
		opts.Executor = NewStdExecutor()
	}

	cmd := opts.Executor.Command(name, opts.Args...)
	cmd.Stdout = opts.Out
	cmd.Stderr = opts.Err
	cmd.Dir = opts.WorkDir

	return cmd
}

func CombinedOutput(cmd *exec.Cmd) (string, error) {
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return string(out), nil
}
