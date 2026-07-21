// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package iox

import (
	"io"
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"
)

const (
	defaultRegionLines = 10
	minRegionWidth     = 20
)

// LiveRegion renders streamed output into an ephemeral, in-place terminal region whose most recent
// lines stay visible while a process runs and that can be wiped clean once it finishes. It is active
// only when attached to a real terminal; otherwise Write and Clear are no-ops so logs stay clean and
// the caller can route the same output elsewhere (e.g. to DEBUG).
type LiveRegion struct {
	w        io.Writer
	enabled  bool
	maxLines int
	width    int
	painted  int
	partial  string
	lines    []string
}

// NewLiveRegion returns a LiveRegion drawing on f. It is enabled only when f is a terminal and the
// caller did not disable it (e.g. --no-tty or debug logging, where raw logs are preferred).
//
//nolint:revive // disable is an explicit opt-out passed by the caller, not an internal mode toggle.
func NewLiveRegion(f *os.File, disable bool) *LiveRegion {
	lr := &LiveRegion{
		w:        f,
		maxLines: defaultRegionLines,
		width:    0,
	}

	if disable || f == nil || !term.IsTerminal(int(f.Fd())) {
		return lr
	}

	lr.enabled = true

	if w, _, err := term.GetSize(int(f.Fd())); err == nil && w > minRegionWidth {
		lr.width = w
	}

	return lr
}

// Enabled reports whether the region actually draws to a terminal.
func (lr *LiveRegion) Enabled() bool {
	return lr.enabled
}

// Write consumes streamed bytes, keeping the most recent lines visible in place.
func (lr *LiveRegion) Write(p []byte) (int, error) {
	if !lr.enabled {
		return len(p), nil
	}

	lr.partial += string(p)

	for {
		idx := strings.IndexByte(lr.partial, '\n')
		if idx < 0 {
			break
		}

		line := strings.TrimRight(lr.partial[:idx], "\r")
		lr.partial = lr.partial[idx+1:]

		lr.lines = append(lr.lines, lr.truncate(line))

		if len(lr.lines) > lr.maxLines {
			lr.lines = lr.lines[len(lr.lines)-lr.maxLines:]
		}
	}

	lr.repaint()

	return len(p), nil
}

// Clear wipes the painted region, leaving the cursor where the region began.
func (lr *LiveRegion) Clear() {
	if !lr.enabled || lr.painted == 0 {
		return
	}

	if _, err := io.WriteString(lr.w, "\033["+strconv.Itoa(lr.painted)+"A\033[J"); err != nil {
		lr.enabled = false
	}

	lr.painted = 0
	lr.lines = nil
	lr.partial = ""
}

// truncate clips a line to the terminal width so it never wraps and breaks the line accounting.
func (lr *LiveRegion) truncate(line string) string {
	if lr.width <= 0 || len(line) <= lr.width {
		return line
	}

	return line[:lr.width-1]
}

func (lr *LiveRegion) repaint() {
	out := ""

	if lr.painted > 0 {
		out += "\033[" + strconv.Itoa(lr.painted) + "A"
	}

	for _, line := range lr.lines {
		out += "\033[2K" + line + "\r\n"
	}

	lr.painted = len(lr.lines)

	if _, err := io.WriteString(lr.w, out); err != nil {
		// The terminal is no longer writable; stop painting so we don't garble output.
		lr.enabled = false
	}
}
