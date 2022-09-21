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

func (e *StdExecutor) Command(name string, arg ...string) *exec.Cmd {
	return exec.Command(name, arg...)
}

func NewFakeExecutor() *FakeExecutor {
	return &FakeExecutor{}
}

type FakeExecutor struct{}

func (e *FakeExecutor) Command(path string, arg ...string) *exec.Cmd {
	name := filepath.Base(path)
	cs := []string{"-test.run=TestHelperProcess", "--", name}
	cs = append(cs, arg...)

	return exec.Command(os.Args[0], cs...)
}
