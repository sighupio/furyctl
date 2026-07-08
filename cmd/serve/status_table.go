// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package serve

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/term"

	execx "github.com/sighupio/furyctl/internal/x/exec"
)

const (
	// Terminal status a node reports once it has booted into the installed OS.
	statusBooted = "booted"
	// Reported when Flatcar is already installed on disk and the installer refuses to
	// overwrite it; needs operator attention.
	statusInstallationBlocked = "installation-blocked"

	// Wall-clock format shown in the "UPDATED" column.
	updatedTimeLayout = "15:04:05"
)

// nodeStatusTable renders a live, in-place table of node bootstrap status on a terminal
// (cursor-up + clear-line repaint), falling back to one log line per update under --debug/--no-tty.
type nodeStatusTable struct {
	mu  sync.Mutex
	out io.Writer
	// True only when the table can be animated; false on non-terminals, --no-tty and --debug.
	tty bool

	order     []string             // Node hostnames in stable (sorted) order.
	status    map[string]string    // Hostname to last reported status.
	updatedAt map[string]time.Time // Hostname to when that status last changed.
	notes     map[string]string    // Hostname to attention line (blocked install); cleared on recovery.

	linesDrawn int // Rows painted by the previous render, so the next one knows how far up to move.
}

// newNodeStatusTable seeds the table from the initial hostname->status map (typically every node at
// "pending"). It is a package var, not a plain func, so tests can swap in a buffer-backed table.
//
//nolint:gochecknoglobals // Overridable constructor so tests can inject a buffer-backed table.
var newNodeStatusTable = func(initial map[string]string) *nodeStatusTable {
	f := os.Stderr

	order := make([]string, 0, len(initial))
	status := make(map[string]string, len(initial))

	for node, st := range initial {
		order = append(order, node)
		status[node] = st
	}

	sort.Strings(order)

	return &nodeStatusTable{
		out:       f,
		tty:       !execx.NoTTY && logrus.GetLevel() < logrus.DebugLevel && term.IsTerminal(int(f.Fd())),
		order:     order,
		status:    status,
		updatedAt: make(map[string]time.Time, len(initial)),
		notes:     make(map[string]string, len(initial)),
	}
}

// Start draws the initial table once, so the operator sees every node (as "pending") the moment the
// server is ready. No-op when not animating.
func (t *nodeStatusTable) Start() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.tty {
		t.render()
	}
}

// Update records a node's new status and repaints (TTY) or logs it (non-TTY).
func (t *nodeStatusTable) Update(node, status string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, known := t.status[node]; !known {
		// A node we didn't seed: keep it visible rather than dropping the update.
		t.order = append(t.order, node)
		sort.Strings(t.order)
	}

	t.status[node] = status
	t.updatedAt[node] = time.Now()

	if status == statusInstallationBlocked {
		t.notes[node] = node + ": Flatcar is already installed on disk; installation blocked, manual intervention required."
	} else {
		// The node moved on from a blocked install: drop its stale attention note.
		delete(t.notes, node)
	}

	if !t.tty {
		if status == statusInstallationBlocked {
			logrus.Errorf(
				"Flatcar Installation on node %s is blocked because Flatcar is already installed on disk. "+
					"Manual intervention required",
				node,
			)
		} else {
			logrus.Infof("Node %s is %s", node, status)
		}

		return
	}

	t.render()
}

// AllBooted reports whether every known node has reached the "booted" state.
func (t *nodeStatusTable) AllBooted() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	if len(t.order) == 0 {
		return false
	}

	for _, st := range t.status {
		if st != statusBooted {
			return false
		}
	}

	return true
}

// Len returns the number of tracked nodes.
func (t *nodeStatusTable) Len() int {
	t.mu.Lock()
	defer t.mu.Unlock()

	return len(t.order)
}

// Snapshot returns a copy of the current hostname->status map for the /status GET endpoint.
func (t *nodeStatusTable) Snapshot() map[string]string {
	t.mu.Lock()
	defer t.mu.Unlock()

	out := make(map[string]string, len(t.status))
	for node, st := range t.status {
		out[node] = st
	}

	return out
}

// bootedCount returns how many nodes are booted. Caller holds t.mu.
func (t *nodeStatusTable) bootedCount() int {
	n := 0

	for _, st := range t.status {
		if st == statusBooted {
			n++
		}
	}

	return n
}

// render repaints the table in place. Caller holds t.mu and t.tty is true.
func (t *nodeStatusTable) render() {
	lines := t.lines()
	prev := t.linesDrawn
	width := t.termWidth()

	out := make([]string, 0, len(lines))

	// Move the cursor back up to the first row of the previously drawn table.
	if prev > 0 {
		out = append(out, cursorUp(prev))
	}

	// \033[2K clears each row; truncate so a wrapped line can't desync the cursor math.
	for _, line := range lines {
		out = append(out, "\033[2K"+truncateLine(line, width)+"\n")
	}

	// The table can shrink when an attention note clears on recovery: erase the now-orphaned rows
	// the previous, larger render left behind, then step back up to just below the current table.
	if extra := prev - len(lines); extra > 0 {
		out = append(out, strings.Repeat("\033[2K\n", extra), cursorUp(extra))
	}

	t.linesDrawn = len(lines)

	_, _ = fmt.Fprint(t.out, strings.Join(out, ""))
}

// cursorUp returns the ANSI escape that moves the cursor up n rows.
func cursorUp(n int) string {
	return "\033[" + strconv.Itoa(n) + "A"
}

// lines builds the full table: a title, the tab-aligned node rows, then any attention notes.
// Caller holds t.mu.
func (t *nodeStatusTable) lines() []string {
	const tabPadding = 2

	// Let tabwriter align the columns; we split its output back into individual lines so the
	// in-place repaint can clear and truncate each row.
	var sb strings.Builder

	w := tabwriter.NewWriter(&sb, 0, 0, tabPadding, ' ', 0)

	_, _ = fmt.Fprintf(w, "NODE\tSTATUS\tUPDATED\n")

	for _, node := range t.order {
		updated := "—"
		if ts, ok := t.updatedAt[node]; ok {
			updated = ts.Format(updatedTimeLayout)
		}

		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", node, t.status[node], updated)
	}

	_ = w.Flush()

	lines := make([]string, 0, len(t.order)+len(t.notes))
	lines = append(lines, fmt.Sprintf("Nodes bootstrap status — %d/%d booted", t.bootedCount(), len(t.order)))

	for _, row := range strings.Split(strings.TrimRight(sb.String(), "\n"), "\n") {
		lines = append(lines, "  "+row)
	}

	// Draw notes in node order (stable), skipping nodes with none.
	for _, node := range t.order {
		if note, ok := t.notes[node]; ok {
			lines = append(lines, "  ! "+note)
		}
	}

	return lines
}

// termWidth returns the terminal width, or 0 when it can't be determined (meaning: don't truncate).
func (t *nodeStatusTable) termWidth() int {
	f, ok := t.out.(*os.File)
	if !ok {
		return 0
	}

	w, _, err := term.GetSize(int(f.Fd()))
	if err != nil || w <= 0 {
		return 0
	}

	return w
}

// truncateLine clips a line to width runes so it can't wrap (a wrap would break the cursor-up count).
func truncateLine(line string, width int) string {
	if width <= 0 {
		return line
	}

	r := []rune(line)
	if len(r) <= width {
		return line
	}

	if width <= 1 {
		return string(r[:width])
	}

	return string(r[:width-1]) + "…"
}
