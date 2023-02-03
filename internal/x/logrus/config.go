// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package logrusx

import (
	"fmt"
	"io"
	"os"

	"github.com/sirupsen/logrus"
)

type LogFormat struct {
	Level  string  `json:"level"`
	Action *string `json:"action,omitempty"`
	Msg    string  `json:"msg"`
	Time   string  `json:"time"`
}

type formatterHook struct {
	Writer    io.Writer
	LogLevels []logrus.Level
	Formatter logrus.Formatter
}

func (hook *formatterHook) Fire(entry *logrus.Entry) error {
	line, err := hook.Formatter.Format(entry)
	if err != nil {
		return fmt.Errorf("error while formatting log entry: %w", err)
	}

	_, err = hook.Writer.Write(line)
	if err != nil {
		return fmt.Errorf("error while writing log entry: %w", err)
	}

	return nil
}

func (hook *formatterHook) Levels() []logrus.Level {
	return hook.LogLevels
}

func newFormatterHook(writer io.Writer, formatter logrus.Formatter, logLevels []logrus.Level) *formatterHook {
	return &formatterHook{
		Writer:    writer,
		Formatter: formatter,
		LogLevels: logLevels,
	}
}

func InitLog(logFile *os.File, debug, disableColors bool) { //nolint:revive // debug is a boolean flag
	logrus.SetOutput(io.Discard)

	stdLevels := []logrus.Level{
		logrus.PanicLevel,
		logrus.FatalLevel,
		logrus.ErrorLevel,
		logrus.WarnLevel,
		logrus.InfoLevel,
	}

	if debug {
		stdLevels = append(stdLevels, logrus.DebugLevel)
		logrus.SetLevel(logrus.DebugLevel)
	}

	outLog := os.Stdout

	if logFile != nil {
		outLog = logFile

		stdOutHook := newFormatterHook(os.Stdout, &logrus.TextFormatter{
			DisableTimestamp: true,
			ForceColors:      !disableColors,
			DisableColors:    disableColors,
		}, stdLevels)

		logrus.AddHook(stdOutHook)
	}

	logFileHook := newFormatterHook(outLog, &logrus.JSONFormatter{}, logrus.AllLevels)

	logrus.AddHook(logFileHook)
}
