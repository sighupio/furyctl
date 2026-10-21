// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/cmd"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

var errCommand = errors.New("error while downloading distribution")

// Exit code and final line of the diff report: the exit 1 of furyctl diff adds no error line after the report.
func TestLogCommandError(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		err  error
		want string
	}{
		"a command error is logged":           {errCommand, "level=error msg=\"error while downloading distribution\"\n"},
		"the diff exit 1 is not logged":       {cmd.ErrDiffNotCompared, ""},
		"a wrapped diff exit 1 is not logged": {fmt.Errorf("diff: %w", cmd.ErrDiffNotCompared), ""},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			out := &bytes.Buffer{}
			log := &logrus.Logger{
				Out:       out,
				Formatter: &logrus.TextFormatter{DisableTimestamp: true, DisableColors: true},
				Level:     logrus.DebugLevel,
			}

			logCommandError(log, tc.err)

			require.Equal(t, tc.want, out.String())
		})
	}
}

// Not parallel: it sets os.Args, execx.LogFile and the standard logger.
// Once the log file is open, the error of a failed command goes to the standard logger, which writes to the
// terminal and to the log file; exec exits with 1.
func TestExecLogsTheErrorToTheStandardLoggerOnceTheLogFileIsOpen(t *testing.T) {
	savedArgs, savedLogFile, savedOut := os.Args, execx.LogFile, logrus.StandardLogger().Out

	t.Cleanup(func() {
		os.Args, execx.LogFile = savedArgs, savedLogFile
		logrus.SetOutput(savedOut)
	})

	logFile, err := os.CreateTemp(t.TempDir(), "furyctl-*.log")
	require.NoError(t, err)

	t.Cleanup(func() { _ = logFile.Close() })

	execx.LogFile = logFile
	os.Args = []string{"furyctl", "--no-such-flag"}

	logs := &bytes.Buffer{}
	logrus.SetOutput(logs)

	require.Equal(t, 1, exec())
	require.Contains(t, logs.String(), "no-such-flag")
}
