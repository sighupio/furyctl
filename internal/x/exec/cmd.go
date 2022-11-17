// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package execx

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

var (
	Debug        = false  //nolint:gochecknoglobals // This variable is shared between all the command instances.
	LogFile      *os.File //nolint:gochecknoglobals // This variable is shared between all the command instances.
	ErrCmdFailed = errors.New("command failed")
)

func NewErrCmdFailed(name string, args []string, err error, res *CmdLog) error {
	return fmt.Errorf("%s %s: %w - %v\n%s", name, strings.Join(args, " "), ErrCmdFailed, err, res)
}

func NewCmd(name string, opts CmdOptions) *Cmd {
	outLog := bytes.NewBufferString("")
	errLog := bytes.NewBufferString("")

	outWriters := []io.Writer{outLog, LogFile}
	errWriters := []io.Writer{errLog, LogFile}

	if opts.Executor == nil {
		opts.Executor = NewStdExecutor()
	}

	if opts.Out != nil {
		outWriters = append(outWriters, opts.Out)
	}

	if opts.Err != nil {
		errWriters = append(errWriters, opts.Err)
	}

	if Debug {
		outWriters = append(outWriters, os.Stdout)
		errWriters = append(errWriters, os.Stderr)
	}

	coreCmd := opts.Executor.Command(name, opts.Args...)
	coreCmd.Stdout = io.MultiWriter(outWriters...)
	coreCmd.Stderr = io.MultiWriter(errWriters...)
	coreCmd.Dir = opts.WorkDir

	return &Cmd{
		Cmd: coreCmd,
		Log: &CmdLog{
			Out: outLog,
			Err: errLog,
		},
	}
}

type Cmd struct {
	*exec.Cmd
	Log *CmdLog
}

func (c *Cmd) Run() error {
	if err := c.Cmd.Run(); err != nil {
		return NewErrCmdFailed(c.Path, c.Args, err, c.Log)
	}

	return nil
}

type CmdOptions struct {
	Args     []string
	Err      io.Writer
	Executor Executor
	Out      io.Writer
	WorkDir  string
}

type CmdLog struct {
	Out *bytes.Buffer
	Err *bytes.Buffer
}

func (c CmdLog) String() string {
	return fmt.Sprintf("out: %s\nerr: %s\n", c.Out, c.Err)
}

func CombinedOutput(cmd *Cmd) (string, error) {
	err := cmd.Run()

	trimOut := strings.Trim(cmd.Log.Out.String(), "\n")
	trimErr := strings.Trim(cmd.Log.Err.String(), "\n")

	return strings.Trim(trimOut+"\n"+trimErr, "\n"), err
}
