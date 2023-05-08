// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package execx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	bytesx "github.com/sighupio/furyctl/internal/x/bytes"
	iox "github.com/sighupio/furyctl/internal/x/io"
)

var (
	Debug         = false  //nolint:gochecknoglobals // This variable is shared between all the command instances.
	LogFile       *os.File //nolint:gochecknoglobals // This variable is shared between all the command instances.
	NoTTY         = false  //nolint:gochecknoglobals // This variable is shared between all the command instances.
	ErrCmdFailed  = errors.New("command failed")
	ErrCmdTimeout = errors.New("command timed out")
)

func NewErrCmdFailed(name string, args []string, err error, res *CmdLog) error {
	return fmt.Errorf("%s %s: %w - %v\n%s", name, strings.Join(args, " "), ErrCmdFailed, err, res)
}

func NewCmd(name string, opts CmdOptions) *Cmd {
	outLog := bytes.NewBufferString("")
	errLog := bytes.NewBufferString("")

	outWriters := []iox.WriterTransform{{W: outLog}}
	errWriters := []iox.WriterTransform{{W: errLog}}

	if LogFile != nil {
		cmd := strings.Split(name, "/")
		cmdArgs := strings.Join(opts.Args, " ")

		action := cmd[len(cmd)-1] + " " + cmdArgs

		stripColor := iox.WriterTransform{
			W: LogFile,
			Transforms: []bytesx.TransformFunc{
				bytesx.StripColor,
				bytesx.ToJSONLogFormat("debug", action),
				bytesx.AppendNewLine,
			},
		}

		outWriters = append(outWriters, stripColor)
		errWriters = append(errWriters, stripColor)
	}

	if opts.Executor == nil {
		opts.Executor = NewStdExecutor()
	}

	if opts.Out != nil {
		outWriters = append(outWriters, iox.WriterTransform{W: opts.Out})
	}

	if opts.Err != nil {
		errWriters = append(errWriters, iox.WriterTransform{W: opts.Err})
	}

	if Debug || LogFile == nil {
		outWriters = append(outWriters, iox.WriterTransform{W: os.Stdout})
		errWriters = append(errWriters, iox.WriterTransform{W: os.Stderr})
	}

	coreCmd := opts.Executor.Command(name, opts.Args...)
	coreCmd.Stdout = iox.MultiWriterTransform(outWriters...)
	coreCmd.Stderr = iox.MultiWriterTransform(errWriters...)
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

func (c *Cmd) Stop() error {
	if c.Process == nil {
		return nil
	}

	if c.ProcessState != nil && c.ProcessState.Exited() {
		return nil
	}

	if err := c.Process.Signal(os.Interrupt); err != nil {
		return fmt.Errorf("failed to interrupt process: %w", err)
	}

	return nil
}

func (c *Cmd) RunWithTimeout(timeout time.Duration) error {
	var cmdCtx *exec.Cmd

	ctx, cancel := context.WithTimeout(context.Background(), timeout)

	defer cancel()

	if len(c.Cmd.Args) == 1 {
		cmdCtx = exec.CommandContext(ctx, c.Cmd.Path)
	} else {
		args := c.Cmd.Args[1:]

		cmdCtx = exec.CommandContext(ctx, c.Cmd.Path, args...)
	}

	cmdCtx.Dir = c.Cmd.Dir
	cmdCtx.Stdout = c.Cmd.Stdout
	cmdCtx.Stderr = c.Cmd.Stderr

	err := cmdCtx.Run()

	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf(
			"%w after %s: %s %s", ErrCmdTimeout, timeout, c.Cmd.Path, strings.Join(c.Cmd.Args, " "),
		)
	}

	if err != nil {
		return NewErrCmdFailed(c.Cmd.Path, c.Cmd.Args, err, c.Log)
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

func CombinedOutputWithTimeout(cmd *Cmd, timeout time.Duration) (string, error) {
	err := cmd.RunWithTimeout(timeout)

	trimOut := strings.Trim(cmd.Log.Out.String(), "\n")
	trimErr := strings.Trim(cmd.Log.Err.String(), "\n")

	return strings.Trim(trimOut+"\n"+trimErr, "\n"), err
}
