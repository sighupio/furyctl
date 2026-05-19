// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package execx

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
)

type Executor interface {
	Command(name string, arg ...string) *exec.Cmd
}

type StdExecutor struct{}

func NewStdExecutor() *StdExecutor {
	return &StdExecutor{}
}

func (*StdExecutor) Command(name string, arg ...string) *exec.Cmd {
	//nolint:noctx // it requires a massive refactor
	return exec.Command(name, arg...)
}

type FakeExecutor struct {
	testHelperProcessFn string
}

func NewFakeExecutor(testHelperProcessFn string) *FakeExecutor {
	if testHelperProcessFn == "" {
		testHelperProcessFn = "TestHelperProcess"
	}

	return &FakeExecutor{
		testHelperProcessFn: testHelperProcessFn,
	}
}

func (fe *FakeExecutor) Command(name string, arg ...string) *exec.Cmd {
	cs := slices.Concat(
		[]string{"-test.run=" + fe.testHelperProcessFn, "--", filepath.Base(name)},
		arg,
	)

	//nolint:gosec,noctx // G702: os.Args[0] is the test binary, never user-controlled. noctx: requires a massive refactor.
	return exec.Command(os.Args[0], cs...)
}
