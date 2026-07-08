// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package serve

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// newTestTable builds a buffer-backed table with animation forced on, so tests can inspect the
// exact bytes a terminal would receive.
func newTestTable(buf *bytes.Buffer, initial map[string]string) *nodeStatusTable {
	real := newNodeStatusTable(initial)
	real.out = buf
	real.tty = true

	return real
}

func TestNodeStatusTableRendersRowsAndCounts(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	table := newTestTable(&buf, map[string]string{"cp1.flatcar": "pending", "cp2.flatcar": "pending"})

	table.Start()

	out := buf.String()
	if !strings.Contains(out, "Nodes bootstrap status — 0/2 booted") {
		t.Fatalf("initial render missing title with counts, got:\n%q", out)
	}

	for _, want := range []string{"NODE", "STATUS", "UPDATED", "cp1.flatcar", "cp2.flatcar", "pending", "—"} {
		if !strings.Contains(out, want) {
			t.Fatalf("initial render missing %q, got:\n%q", want, out)
		}
	}
}

func TestNodeStatusTableUpdateRepaintsInPlace(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	table := newTestTable(&buf, map[string]string{"cp1.flatcar": "pending", "cp2.flatcar": "pending"})

	table.Start() // title + header + 2 rows = 4 lines.

	buf.Reset()
	table.Update("cp1.flatcar", statusBooted)

	out := buf.String()

	// The repaint must move the cursor up over the 4 previously drawn lines before redrawing.
	if !strings.Contains(out, "\033[4A") {
		t.Fatalf("repaint did not move cursor up 4 lines, got:\n%q", out)
	}

	if !strings.Contains(out, "Nodes bootstrap status — 1/2 booted") {
		t.Fatalf("repaint did not update booted count, got:\n%q", out)
	}

	if !strings.Contains(out, "booted") {
		t.Fatalf("repaint did not show booted status, got:\n%q", out)
	}
}

func TestNodeStatusTableAllBooted(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	table := newTestTable(&buf, map[string]string{"cp1.flatcar": "pending", "cp2.flatcar": "pending"})

	if table.AllBooted() {
		t.Fatal("AllBooted must be false while nodes are pending")
	}

	table.Update("cp1.flatcar", statusBooted)

	if table.AllBooted() {
		t.Fatal("AllBooted must be false while one node is still pending")
	}

	table.Update("cp2.flatcar", statusBooted)

	if !table.AllBooted() {
		t.Fatal("AllBooted must be true once every node is booted")
	}
}

func TestNodeStatusTableAllBootedEmpty(t *testing.T) {
	t.Parallel()

	table := newTestTable(&bytes.Buffer{}, map[string]string{})

	if table.AllBooted() {
		t.Fatal("AllBooted must be false with no nodes")
	}
}

func TestNodeStatusTableBlockedInstallAddsNote(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	table := newTestTable(&buf, map[string]string{"cp1.flatcar": "pending"})

	table.Start()
	buf.Reset()
	table.Update("cp1.flatcar", statusInstallationBlocked)

	out := buf.String()
	if !strings.Contains(out, "manual intervention required") {
		t.Fatalf("blocked install did not surface an attention note, got:\n%q", out)
	}
}

// replayANSI applies the subset of terminal control sequences the table emits (cursor-up "\033[nA",
// clear-line "\033[2K", newline) to reconstruct the final visible screen, so tests can assert what a
// terminal would actually show after a series of in-place repaints. Returns non-empty screen lines.
func replayANSI(data string) []string {
	var screen []string

	row := 0

	ensure := func(r int) {
		for len(screen) <= r {
			screen = append(screen, "")
		}
	}

	for i := 0; i < len(data); {
		switch {
		case strings.HasPrefix(data[i:], "\033["):
			j := i + 2
			for j < len(data) && data[j] >= '0' && data[j] <= '9' {
				j++
			}

			num := 0
			if j > i+2 {
				_, _ = fmt.Sscanf(data[i+2:j], "%d", &num)
			}

			switch data[j] {
			case 'A':
				if row -= num; row < 0 {
					row = 0
				}
			case 'K':
				ensure(row)
				screen[row] = ""
			}

			i = j + 1

		case data[i] == '\n':
			row++

			ensure(row)

			i++

		default:
			k := i
			for k < len(data) && data[k] != '\n' && data[k] != '\033' {
				k++
			}

			ensure(row)

			screen[row] += data[i:k]
			i = k
		}
	}

	out := make([]string, 0, len(screen))

	for _, line := range screen {
		if strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}

	return out
}

func TestNodeStatusTableShrinksCleanlyWithMultipleNotes(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	table := newTestTable(&buf, map[string]string{
		"cp1.flatcar": "pending", "cp2.flatcar": "pending", "cp3.flatcar": "pending",
		"node1.flatcar": "pending", "node2.flatcar": "pending",
	})

	table.Start()

	// Two nodes hit the blocked state, then both are intervened on and boot.
	table.Update("cp1.flatcar", statusInstallationBlocked)
	table.Update("node1.flatcar", statusInstallationBlocked)

	// Reproduce the reported bug: after the notes appear, replay the full stream and confirm both.
	if blocked := countLinesContaining(replayANSI(buf.String()), "manual intervention required"); blocked != 2 {
		t.Fatalf("expected 2 distinct attention notes while blocked, got %d", blocked)
	}

	table.Update("cp1.flatcar", statusBooted)
	table.Update("node1.flatcar", statusBooted)

	screen := replayANSI(buf.String())

	if blocked := countLinesContaining(screen, "manual intervention required"); blocked != 0 {
		t.Fatalf("attention notes lingered after both nodes recovered:\n%s", strings.Join(screen, "\n"))
	}

	// No node row must be duplicated on the final screen.
	if dup := countLinesContaining(screen, "node1.flatcar"); dup != 1 {
		t.Fatalf("node1.flatcar row appears %d times, expected exactly 1:\n%s", dup, strings.Join(screen, "\n"))
	}
}

func countLinesContaining(lines []string, sub string) int {
	n := 0

	for _, line := range lines {
		if strings.Contains(line, sub) {
			n++
		}
	}

	return n
}

func TestNodeStatusTableNoteClearedOnRecovery(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	table := newTestTable(&buf, map[string]string{"cp1.flatcar": "pending"})

	table.Update("cp1.flatcar", statusInstallationBlocked)

	// The node was intervened on and now boots: its attention note must disappear.
	buf.Reset()
	table.Update("cp1.flatcar", statusBooted)

	if out := buf.String(); strings.Contains(out, "manual intervention required") {
		t.Fatalf("attention note lingered after the node recovered, got:\n%q", out)
	}
}

func TestNodeStatusTableSnapshotIsACopy(t *testing.T) {
	t.Parallel()

	table := newTestTable(&bytes.Buffer{}, map[string]string{"cp1.flatcar": "pending"})

	snap := table.Snapshot()
	snap["cp1.flatcar"] = "tampered"

	if again := table.Snapshot(); again["cp1.flatcar"] != "pending" {
		t.Fatalf("Snapshot must return an independent copy, got %q", again["cp1.flatcar"])
	}
}

func TestTruncateLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		line  string
		width int
		want  string
	}{
		{"no truncation when width is zero", "hello world", 0, "hello world"},
		{"no truncation when it fits", "hello", 10, "hello"},
		{"truncates with ellipsis", "hello world", 5, "hell…"},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := truncateLine(tt.line, tt.width); got != tt.want {
				t.Fatalf("truncateLine(%q, %d) = %q, want %q", tt.line, tt.width, got, tt.want)
			}
		})
	}
}
