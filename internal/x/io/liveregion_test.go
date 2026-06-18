// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package iox //nolint:testpackage // exercises unexported region state.

import (
	"bytes"
	"strings"
	"testing"
)

func Test_LiveRegion_Disabled(t *testing.T) {
	t.Parallel()

	lr := NewLiveRegion(nil, true)

	if lr.Enabled() {
		t.Fatal("expected region to be disabled with a nil file")
	}

	n, err := lr.Write([]byte("some output\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if n != len("some output\n") {
		t.Fatalf("expected Write to report all bytes consumed, got %d", n)
	}

	// Clear on a disabled region must be a no-op and must not panic.
	lr.Clear()
}

func Test_LiveRegion_RendersAndClears(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	lr := &LiveRegion{w: buf, enabled: true, maxLines: defaultRegionLines}

	if _, err := lr.Write([]byte("mise kubectl@1.34.4 ok\nmise helm@3.19.0 ok\n")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"mise kubectl@1.34.4 ok", "mise helm@3.19.0 ok", "\033["} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected painted output to contain %q, got %q", want, out)
		}
	}

	if lr.painted != 2 {
		t.Fatalf("expected 2 painted lines, got %d", lr.painted)
	}

	buf.Reset()
	lr.Clear()

	if !strings.Contains(buf.String(), "\033[2A\033[J") {
		t.Fatalf("expected Clear to emit cursor-up + erase sequence, got %q", buf.String())
	}

	if lr.painted != 0 {
		t.Fatalf("expected painted reset to 0 after Clear, got %d", lr.painted)
	}
}

func Test_LiveRegion_MaxLines(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	lr := &LiveRegion{w: buf, enabled: true, maxLines: 2}

	if _, err := lr.Write([]byte("l1\nl2\nl3\nl4\n")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(lr.lines) != 2 {
		t.Fatalf("expected only the last 2 lines retained, got %d", len(lr.lines))
	}

	if lr.lines[0] != "l3" || lr.lines[1] != "l4" {
		t.Fatalf("expected last lines [l3 l4], got %v", lr.lines)
	}
}

func Test_LiveRegion_Truncate(t *testing.T) {
	t.Parallel()

	lr := &LiveRegion{width: 10}

	if got := lr.truncate("short"); got != "short" {
		t.Fatalf("expected short line untouched, got %q", got)
	}

	got := lr.truncate("abcdefghijklmnop")
	if len(got) != 9 {
		t.Fatalf("expected line clipped to width-1 (9), got %d (%q)", len(got), got)
	}
}
