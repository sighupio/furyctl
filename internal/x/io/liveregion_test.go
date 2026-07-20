// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package iox //nolint:testpackage // exercises unexported region state.

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_LiveRegion_Disabled(t *testing.T) {
	t.Parallel()

	lr := NewLiveRegion(nil, true)

	require.False(t, lr.Enabled(), "expected region to be disabled with a nil file")

	n, err := lr.Write([]byte("some output\n"))
	require.NoError(t, err)
	require.Equal(t, len("some output\n"), n, "expected Write to report all bytes consumed")

	// Clear on a disabled region must be a no-op and must not panic.
	lr.Clear()
}

func Test_LiveRegion_RendersAndClears(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	lr := &LiveRegion{w: buf, enabled: true, maxLines: defaultRegionLines}

	_, err := lr.Write([]byte("mise kubectl@1.34.4 ok\nmise helm@3.19.0 ok\n"))
	require.NoError(t, err)

	out := buf.String()
	for _, want := range []string{"mise kubectl@1.34.4 ok", "mise helm@3.19.0 ok", "\033["} {
		require.Contains(t, out, want, "expected painted output to contain %q, got %q", want, out)
	}

	require.Equal(t, 2, lr.painted, "expected 2 painted lines")

	buf.Reset()
	lr.Clear()

	require.Contains(t, buf.String(), "\033[2A\033[J", "expected Clear to emit cursor-up + erase sequence, got %q", buf.String())
	require.Equal(t, 0, lr.painted, "expected painted reset to 0 after Clear")
}

func Test_LiveRegion_MaxLines(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	lr := &LiveRegion{w: buf, enabled: true, maxLines: 2}

	_, err := lr.Write([]byte("l1\nl2\nl3\nl4\n"))
	require.NoError(t, err)

	require.Len(t, lr.lines, 2, "expected only the last 2 lines retained")
	require.Equal(t, []string{"l3", "l4"}, lr.lines, "expected last lines [l3 l4]")
}

func Test_LiveRegion_Truncate(t *testing.T) {
	t.Parallel()

	lr := &LiveRegion{width: 10}

	got := lr.truncate("short")
	require.Equal(t, "short", got, "expected short line untouched")

	got = lr.truncate("abcdefghijklmnop")
	require.Len(t, got, 9, "expected line clipped to width-1 (9), got %q", got)
}
