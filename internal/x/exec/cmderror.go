// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package execx

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	bytesx "github.com/sighupio/furyctl/internal/x/bytes"
	iox "github.com/sighupio/furyctl/internal/x/io"
)

// OutputLines is how many non-empty lines of the output the error of a failed command shows.
const OutputLines = 10

// Line is a line of the output of a command.
type Line struct {
	Text   string
	Stderr bool
}

// CmdError is the error of a failed command. Run fills it with the defaults, then gives it to the
// OnError function of the command, then saves Text to a file in LogDir.
type CmdError struct {
	// Err is the error of os/exec, for example "exit status 2".
	Err error
	// Name is the program that the error shows, without its path.
	Name string
	Args []string
	// Lines is the output, stdout and stderr in arrival order.
	Lines []Line
	// Text is what the output file holds. With no Text, Run saves no file.
	Text []string
	// LogName is the start of the output file name. Empty means Name and the first of Args.
	LogName string
	// Summary comes before the command line.
	Summary string
	LeadIn  string
	// Block is the part of the output that the error shows.
	Block []string
	// Note comes after the block.
	Note string

	command string
	file    string
}

func (e *CmdError) Error() string {
	var b strings.Builder

	if e.Summary != "" {
		b.WriteString(e.Summary + "\n\n")
	}

	fmt.Fprintf(&b, "%s: %s (%v)", ErrCmdFailed, e.command, e.Err)

	if len(e.Block) > 0 {
		b.WriteString("\n\n" + e.LeadIn)

		for _, l := range e.Block {
			b.WriteString(strings.TrimRight("\n │ "+l, " \t"))
		}

		if e.Note != "" {
			b.WriteString("\n" + e.Note)
		}
	}

	if e.file != "" {
		b.WriteString("\n\nThe full output of the command is in " + e.file)
	}

	return b.String()
}

func (e *CmdError) Unwrap() []error {
	return []error{ErrCmdFailed, e.Err}
}

func newCmdError(c *Cmd, err error) *CmdError {
	lines := c.Log.lines.all()
	text := make([]string, len(lines))

	for i, l := range lines {
		text[i] = l.Text
	}

	e := &CmdError{
		Err:    err,
		Name:   filepath.Base(c.Path),
		Args:   c.Args[1:],
		Lines:  lines,
		Text:   text,
		LeadIn: "Last lines of the command output:",
		Block:  Tail(text, OutputLines),
	}

	if c.OnError != nil {
		c.OnError(e)
	}

	args := make([]string, len(e.Args))
	for i, a := range e.Args {
		args[i] = relPath(a)
	}

	e.command = strings.Join(append([]string{e.Name}, args...), " ")

	if len(e.Text) > 0 && !c.Probe {
		e.file = e.save()
	}

	return e
}

// save writes Text to a file in LogDir and returns its path. If it cannot, it returns the furyctl log
// file, which holds the output too.
func (e *CmdError) save() string {
	name := e.LogName
	if name == "" {
		name = e.Name
		if len(e.Args) > 0 && !strings.HasPrefix(e.Args[0], "-") {
			name += "-" + filepath.Base(e.Args[0])
		}
	}

	file, err := saveOutput(LogDir, name, time.Now(), e.Text)
	if err == nil {
		return relPath(file)
	}

	logrus.Debugf("furyctl cannot save the output of the command: %v", err)

	if LogFile != nil {
		return relPath(LogFile.Name())
	}

	return ""
}

func saveOutput(dir, name string, now time.Time, text []string) (string, error) {
	if dir == "" {
		return "", ErrNoLogDir
	}

	safe := strings.Map(func(r rune) rune {
		if r == '.' || r == '-' || r == '_' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' {
			return r
		}

		return '_'
	}, name)

	file := filepath.Join(dir, safe+"-"+now.Format("20060102-150405.000")+".log")

	if err := os.MkdirAll(dir, iox.UserGroupPerm); err != nil {
		return "", fmt.Errorf("error creating the log folder: %w", err)
	}

	if err := os.WriteFile(file, []byte(strings.Join(text, "\n")+"\n"), iox.FullRWPermAccess); err != nil {
		return "", fmt.Errorf("error writing the output file: %w", err)
	}

	return file, nil
}

// Tail returns the last n non-empty lines, with the empty lines between them.
func Tail(lines []string, n int) []string {
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}

	i := len(lines)

	for ; i > 0 && n > 0; i-- {
		if strings.TrimSpace(lines[i-1]) != "" {
			n--
		}
	}

	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}

	return lines[i:]
}

// StderrBlock is an OnError hook for a tool whose stdout is data, for example JSON or YAML, and whose
// errors go to stderr. When the tool wrote to stderr, the block shows only the end of stderr: in arrival
// order the data can come last and push the error out of the block.
func StderrBlock(e *CmdError) {
	var stderr []string

	for _, l := range e.Lines {
		if l.Stderr {
			stderr = append(stderr, l.Text)
		}
	}

	if block := Tail(stderr, OutputLines); len(block) > 0 {
		e.Block = block
	}
}

// relPath returns p relative to the current directory when p is an absolute path under it.
func relPath(p string) string {
	if !filepath.IsAbs(p) {
		return p
	}

	wd, err := os.Getwd()
	if err != nil {
		return p
	}

	rel, err := filepath.Rel(wd, p)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return p
	}

	return rel
}

// A lineLog keeps the lines of stdout and stderr in arrival order. The lock is necessary, because
// each stream has a goroutine of its own in os/exec.
type lineLog struct {
	mu      sync.Mutex
	lines   []Line
	partial [2][]byte
}

type lineWriter struct {
	log    *lineLog
	stderr bool
}

func (w lineWriter) Write(p []byte) (int, error) {
	l := w.log
	l.mu.Lock()
	defer l.mu.Unlock()

	s := 0
	if w.stderr {
		s = 1
	}

	l.partial[s] = append(l.partial[s], p...)

	for {
		i := bytes.IndexByte(l.partial[s], '\n')
		if i < 0 {
			return len(p), nil
		}

		l.add(l.partial[s][:i], w.stderr)
		l.partial[s] = l.partial[s][i+1:]
	}
}

func (l *lineLog) add(p []byte, stderr bool) {
	if s, err := bytesx.StripColor(p); err == nil {
		p = s
	}

	l.lines = append(l.lines, Line{Text: strings.TrimSuffix(string(p), "\r"), Stderr: stderr})
}

// all returns the lines, with the last line of each stream when it has no newline.
func (l *lineLog) all() []Line {
	if l == nil {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	for s, p := range l.partial {
		if len(p) > 0 {
			l.add(p, s == 1)
			l.partial[s] = nil
		}
	}

	return l.lines
}
