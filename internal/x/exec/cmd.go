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
	"syscall"

	bytesx "github.com/sighupio/furyctl/internal/x/bytes"
	iox "github.com/sighupio/furyctl/internal/x/io"
)

var (
	Debug              = false  //nolint:gochecknoglobals // This variable is shared between all the command instances.
	LogFile            *os.File //nolint:gochecknoglobals // This variable is shared between all the command instances.
	NoTTY              = false  //nolint:gochecknoglobals // This variable is shared between all the command instances.
	ErrCmdFailed       = errors.New("command failed")
	ErrCastingToBuffer = errors.New("error casting stdout to bytes.Buffer")
)

func NewErrCmdFailed(name string, args []string, err error, res *CmdLog) error {
	return fmt.Errorf("%s %s: %w - %v\n%s", name, strings.Join(args, " "), ErrCmdFailed, err, res)
}

type Cmd struct {
	*exec.Cmd

	Log       *CmdLog
	Sensitive bool
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

	// A process group of its own lets stopAll reach the descendants too, for example the ansible workers.
	if !opts.Foreground {
		coreCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}

	// Extra env vars are appended to the inherited environment (used e.g. to invoke the bundled mise
	// hermetically: MISE_DATA_DIR, MISE_GLOBAL_CONFIG_FILE, ...).
	if len(opts.Env) > 0 {
		coreCmd.Env = append(os.Environ(), opts.Env...)
	}

	if opts.Sensitive {
		coreCmd.Stdout = bytes.NewBufferString("")
		coreCmd.Stderr = bytes.NewBufferString("")
	}

	return &Cmd{
		Cmd: coreCmd,
		Log: &CmdLog{
			Out: outLog,
			Err: errLog,
		},
		Sensitive: opts.Sensitive,
	}
}

func (c *Cmd) Run() error {
	if err := run(c.Cmd); err != nil {
		return NewErrCmdFailed(c.Path, c.Args, err, c.Log)
	}

	return nil
}

func (c *Cmd) Stop() error {
	if c.Process == nil {
		return nil
	}

	// Signal 0 fails after Wait reaps the process, so kill cannot hit a reused process group.
	if err := c.Process.Signal(syscall.Signal(0)); err != nil {
		return nil //nolint:nilerr // The process is already gone.
	}

	if err := kill(c.Cmd, syscall.SIGINT); err != nil {
		return fmt.Errorf("failed to interrupt process: %w", err)
	}

	return nil
}

type CmdOptions struct {
	Args     []string
	Env      []string
	Err      io.Writer
	Executor Executor
	// Foreground keeps the command in the process group of furyctl, so it can read the terminal,
	// for example for the sudo password prompt.
	Foreground bool
	Out        io.Writer
	Sensitive  bool
	WorkDir    string
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

	out := cmd.Log.Out.String()
	errOut := cmd.Log.Err.String()

	if cmd.Sensitive {
		outB, ok := cmd.Stdout.(*bytes.Buffer)
		if !ok {
			return "", ErrCastingToBuffer
		}

		errOutB, ok := cmd.Stderr.(*bytes.Buffer)
		if !ok {
			return "", ErrCastingToBuffer
		}

		out = outB.String()
		errOut = errOutB.String()
	}

	trimOut := strings.Trim(out, "\n")
	trimErr := strings.Trim(errOut, "\n")

	return strings.Trim(trimOut+"\n"+trimErr, "\n"), err
}
