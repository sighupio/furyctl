// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package execx

import (
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

func (*StdExecutor) Command(name string, arg ...string) *exec.Cmd {
	return exec.Command(name, arg...)
}

func NewFakeExecutor(testHelperProcessFn string) *FakeExecutor {
	if testHelperProcessFn == "" {
		testHelperProcessFn = "TestHelperProcess"
	}

	return &FakeExecutor{
		testHelperProcessFn: testHelperProcessFn,
	}
}

type FakeExecutor struct {
	testHelperProcessFn string
}

func (fe *FakeExecutor) Command(name string, arg ...string) *exec.Cmd {
	cs := []string{"-test.run=" + fe.testHelperProcessFn, "--", filepath.Base(name)}
	cs = append(cs, arg...)

	return exec.Command(os.Args[0], cs...)
}
