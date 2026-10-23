// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package statusx reports the state of a long-running command to the terminal, with the OSC 7501
// Program Status Protocol and the older OSC 9;4 progress sequence. Terminals ignore the sequences
// that they do not support, and a terminal with OSC 7501 support ignores OSC 9;4 after the first
// OSC 7501 report. See https://www.superlogical.com/rex/docs/build/program-status.
package statusx

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"

	"golang.org/x/term"
)

const (
	app = "furyctl"
	// Bytes, below the 192 bytes for title and 2048 bytes for msg that the protocol allows. A
	// longer value makes the terminal discard the whole report.
	maxBytes = 160
	// Sent with no progress value: indeterminate.
	noProgress  = -1
	maxProgress = 100
	// Ghostty removes an OSC 9;4 bar after 15 s with no update.
	barRefresh = 5 * time.Second
)

//nolint:gochecknoglobals // One terminal per process: commands and prompts share this state.
var state = struct {
	sync.Mutex

	out       io.Writer // Nil: reports are off.
	title     string
	msg       string // The last Working report, sent again by Resume.
	progress  int
	cancelled bool
	bar       string   // The last OSC 9;4 sequence while the command runs, sent again by the timer.
	phases    []string // The phases of the command, in order. Each has an equal part of the progress.
	phase     int      // The index in phases of the current phase, or -1 before the first.
}{progress: noProgress, phase: -1}

// Start turns the reports on for a long-running command, when stderr is a terminal and the
// operator did not set --no-tty. It sends nothing. Commands that do not call Start send no reports.
//
//nolint:revive // noTTY is the operator's --no-tty flag.
func Start(title string, noTTY bool) {
	if noTTY || !term.IsTerminal(int(os.Stderr.Fd())) {
		return
	}

	state.Lock()
	state.out = os.Stderr
	state.title = title
	state.Unlock()

	// ponytail: this write can land between two chunks of the raw output of a command (only with
	// --debug) and split an ANSI sequence there. Rare, and only the look of that one line changes.
	go func() {
		for range time.Tick(barRefresh) {
			state.Lock()
			//nolint:errcheck,revive // A status report is not worth failing the command for.
			io.WriteString(state.out, state.bar)
			state.Unlock()
		}
	}()
}

// SetPhases gives the main phases that the command runs, in order. Then the progress is the
// progress of the whole command (see overall).
func SetPhases(phases ...string) {
	state.Lock()
	defer state.Unlock()

	state.phases = phases
}

// SetTitle changes the title, for example when the cluster name is known, and sends the last
// Working report again with it.
func SetTitle(title string) {
	state.Lock()
	defer state.Unlock()

	state.title = title
	send("working", "", state.msg, state.progress)
}

// Working reports that the command runs. A negative progress means indeterminate.
func Working(msg string, progress int) {
	state.Lock()
	defer state.Unlock()

	state.msg, state.progress = msg, overall(progress)
	send("working", "", msg, state.progress)
}

// Phase reports the start of a phase. A phase of the list that Start got moves the progress to
// the start of its part. A phase out of the list, for example the preflight, keeps the progress.
func Phase(name string) {
	state.Lock()
	defer state.Unlock()

	// The index only goes up: the distribution phase that runs again after the plugins phase stays
	// in the part of the plugins phase.
	if i := slices.Index(state.phases, name); i > state.phase {
		state.phase = i
	}

	state.msg, state.progress = "Running the "+name+" phase", overall(noProgress)
	send("working", "", state.msg, state.progress)
}

// Step reports that the command runs step i (from 0) of n steps.
func Step(msg string, i, n int) {
	Working(msg, i*maxProgress/max(n, 1))
}

// Detail reports a step inside the last Working report, for example one download or one task.
// Resume goes back to the message of the last Working report, with the progress of Detail. A
// negative progress keeps the progress.
func Detail(msg string, progress int) {
	state.Lock()
	defer state.Unlock()

	if progress >= 0 {
		state.progress = overall(progress)
	}

	send("working", "", msg, state.progress)
}

// Blocked reports that the command waits for an action of the operator, for example a machine boot.
func Blocked(msg string) {
	state.Lock()
	defer state.Unlock()

	send("blocked", "", msg, state.progress)
}

// Question reports that the command waits for an answer from the operator.
func Question(msg string) {
	state.Lock()
	defer state.Unlock()

	send("blocked", "question", msg, state.progress)
}

// Resume sends the last Working report again, after a Blocked report.
func Resume() {
	state.Lock()
	defer state.Unlock()

	send("working", "", state.msg, state.progress)
}

// Idle reports that the operator cancelled the command. No report comes after it: a download
// closed or a command line printed while the commands stop must not replace it.
func Idle(msg string) {
	state.Lock()
	defer state.Unlock()

	send("idle", "", msg, noProgress)
	state.cancelled = true
}

func Done(msg string) {
	final("done", msg)
}

func Error(msg string) {
	final("error", msg)
}

func final(st, msg string) {
	state.Lock()
	defer state.Unlock()

	send(st, "", msg, noProgress)
}

// overall maps the progress of a step (0-100, or negative for none) to the progress of the
// command. Each phase has an equal part, and the step fills the part of the current phase. The
// result never goes back. Without phases, it is the progress of the step. Call it with the lock held.
//
// ponytail: equal parts, so the bar stays still in a long phase with no measured steps. Weigh the
// parts by the usual time of each phase if this looks wrong.
func overall(progress int) int {
	n := len(state.phases)
	if n == 0 {
		return progress
	}

	if state.phase < 0 {
		return noProgress
	}

	return max((state.phase*maxProgress+min(max(progress, 0), maxProgress))/n, state.progress)
}

// send must be called with the lock held.
func send(st, kind, msg string, progress int) {
	if state.out == nil || state.cancelled {
		return
	}

	state.bar = ""
	if st == "working" || st == "blocked" {
		state.bar = bar(st, progress)
	}

	//nolint:errcheck,revive // A status report is not worth failing the command for.
	io.WriteString(state.out, report(st, kind, state.title, msg, progress))
}

// report returns the OSC 7501 report and the OSC 9;4 sequence for a state.
func report(st, kind, title, msg string, progress int) string {
	var b strings.Builder

	fmt.Fprintf(&b, "\x1b]7501;state=%s:app=%s", st, app)

	if kind != "" {
		b.WriteString(":kind=" + kind)
	}

	if progress >= 0 && (st == "working" || st == "blocked") {
		fmt.Fprintf(&b, ":progress=%d", min(progress, maxProgress))
	}

	if t := clean(title); t != "" {
		b.WriteString(":title=" + base64.StdEncoding.EncodeToString([]byte(t)))
	}

	if m := clean(msg); m != "" {
		b.WriteString(":msg=" + base64.StdEncoding.EncodeToString([]byte(m)))
	}

	b.WriteString("\x1b\\")

	return b.String() + bar(st, progress)
}

// bar returns the OSC 9;4 sequence for a state. States: 0 remove, 1 progress, 2 error,
// 3 indeterminate, 4 paused.
func bar(st string, progress int) string {
	switch {
	// A bar at 0 % has no width, and the terminal shows nothing (iTerm2). Thus 0 % is indeterminate
	// when working, and a full bar when blocked.
	case st == "working" && progress > 0:
		return fmt.Sprintf("\x1b]9;4;1;%d\x07", min(progress, maxProgress))
	case st == "working":
		return "\x1b]9;4;3\x07"
	case st == "blocked" && progress > 0:
		return fmt.Sprintf("\x1b]9;4;4;%d\x07", min(progress, maxProgress))
	case st == "blocked":
		return "\x1b]9;4;4;100\x07"
	case st == "error":
		return "\x1b]9;4;2;100\x07"
	default:
		return "\x1b]9;4;0\x07"
	}
}

// clean keeps the first line of s, without control characters, because the terminal discards a
// report whose text has one. It also shortens the text.
func clean(s string) string {
	s, _, _ = strings.Cut(strings.TrimLeft(s, " \n"), "\n")
	s = strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}

		return r
	}, s))

	if len(s) > maxBytes {
		// ToValidUTF8 removes a rune that the cut splits.
		s = strings.ToValidUTF8(s[:maxBytes-len("…")], "") + "…"
	}

	return s
}
