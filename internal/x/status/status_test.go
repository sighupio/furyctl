// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package statusx //nolint:testpackage // exercises the unexported report and state.

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
)

func Test_Report(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		state    string
		kind     string
		title    string
		msg      string
		progress int
		want     string
	}{
		{
			name:     "working with progress",
			state:    "working",
			title:    "apply lab",
			msg:      "Upgrading",
			progress: 37,
			want: "\x1b]7501;state=working:app=furyctl:progress=37:title=YXBwbHkgbGFi:msg=VXBncmFkaW5n\x1b\\" +
				"\x1b]9;4;1;37\x07",
		},
		{
			name:     "working without progress",
			state:    "working",
			progress: noProgress,
			want:     "\x1b]7501;state=working:app=furyctl\x1b\\\x1b]9;4;3\x07",
		},
		{
			name:     "working at 0 % shows an indeterminate bar",
			state:    "working",
			progress: 0,
			want:     "\x1b]7501;state=working:app=furyctl:progress=0\x1b\\\x1b]9;4;3\x07",
		},
		{
			name:     "question keeps the progress",
			state:    "blocked",
			kind:     "question",
			progress: 50,
			want:     "\x1b]7501;state=blocked:app=furyctl:kind=question:progress=50\x1b\\\x1b]9;4;4;50\x07",
		},
		{
			name:     "blocked at 0 % shows a full paused bar",
			state:    "blocked",
			progress: 0,
			want:     "\x1b]7501;state=blocked:app=furyctl:progress=0\x1b\\\x1b]9;4;4;100\x07",
		},
		{
			name:     "error drops the progress and the lines after the first",
			state:    "error",
			msg:      "boom\nstack",
			progress: 50,
			want:     "\x1b]7501;state=error:app=furyctl:msg=Ym9vbQ==\x1b\\\x1b]9;4;2;100\x07",
		},
		{
			name:     "done removes the OSC 9;4 bar",
			state:    "done",
			progress: noProgress,
			want:     "\x1b]7501;state=done:app=furyctl\x1b\\\x1b]9;4;0\x07",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, report(tc.state, tc.kind, tc.title, tc.msg, tc.progress))
		})
	}
}

func Test_Clean(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "ab", clean(" a\tb \n c"))
	assert.Equal(t, strings.Repeat("a", maxBytes), clean(strings.Repeat("a", maxBytes)))

	long := clean(strings.Repeat("é", 500))
	assert.LessOrEqual(t, len(long), maxBytes)
	assert.True(t, utf8.ValidString(long))
	assert.True(t, strings.HasSuffix(long, "é…"))
}

//nolint:paralleltest // changes the package state.
func Test_IdleIsTheLastReport(t *testing.T) {
	var buf bytes.Buffer

	state.out = &buf
	t.Cleanup(func() { state.out, state.cancelled, state.msg = nil, false, "" })

	Working("Downloading the distribution", noProgress)
	Idle("Cancelled")
	Resume()
	Detail("TASK [kubeadm : Upgrade the node]", noProgress)
	Phase("kubernetes")
	Error("error reading user input")
	Done("Configuration applied")

	assert.Equal(t, 2, strings.Count(buf.String(), "\x1b]7501;"))
	assert.True(t, strings.HasSuffix(buf.String(), "\x1b]9;4;0\x07"), "idle is the last report")
	assert.Contains(t, buf.String(), "state=idle")
	assert.Empty(t, state.bar, "the timer stops after a cancel")
}

//nolint:paralleltest // changes the package state.
func Test_DoneStopsTheBar(t *testing.T) {
	var buf bytes.Buffer

	state.out = &buf
	t.Cleanup(func() { state.out, state.msg = nil, "" })

	Working("Running the kubernetes phase", noProgress)
	assert.Equal(t, "\x1b]9;4;3\x07", state.bar, "the timer sends the bar again while the command runs")

	Done("Configuration applied")

	assert.Contains(t, buf.String(), "state=done")
	assert.Empty(t, state.bar, "the timer stops after the command ends")
}

//nolint:paralleltest // changes the package state.
func Test_Scanner(t *testing.T) {
	var buf bytes.Buffer

	state.out = &buf
	t.Cleanup(func() { state.out, state.msg, state.progress = nil, "", noProgress })

	Working("Upgrading the worker node node1 (2 of 4)", 25)

	s := &Scanner{}

	for _, chunk := range []string{
		"10:20:30AM: ---- waiting on 3 chan", "ges [53/56 done] ----\n",
		"TASK [kubeadm : Upgrade the node] ***\r\n",
		"no progress here\npartial line",
	} {
		_, err := s.Write([]byte(chunk))
		assert.NoError(t, err)
	}

	msg := func(m string) string { return "msg=" + base64.StdEncoding.EncodeToString([]byte(m)) }

	assert.Contains(t, buf.String(), "progress=94:"+msg("Waiting for the changes: 53 of 56 done"))
	// A task keeps the progress.
	assert.Contains(t, buf.String(), "progress=94:"+msg("kubeadm : Upgrade the node"))
	assert.Equal(t, 3, strings.Count(buf.String(), "\x1b]7501;"))

	buf.Reset()
	Resume()

	assert.Contains(t, buf.String(), msg("Upgrading the worker node node1 (2 of 4)"))
}

//nolint:paralleltest // changes the package state.
func Test_OverallProgress(t *testing.T) {
	var buf bytes.Buffer

	state.out, state.phases = &buf, []string{"infrastructure", "kubernetes", "distribution", "plugins"}
	t.Cleanup(func() { state.out, state.phases, state.phase, state.msg, state.progress = nil, nil, -1, "", noProgress })

	Phase("preflight")
	assert.Equal(t, noProgress, state.progress, "no phase of the list has started")

	Phase("kubernetes")
	assert.Equal(t, 25, state.progress)

	Step("Upgrading the worker node", 1, 2)
	assert.Equal(t, 37, state.progress, "half of the kubernetes part")

	Phase("kubernetes")
	assert.Equal(t, 37, state.progress, "the progress never goes back")

	Phase("plugins")
	assert.Equal(t, 75, state.progress)

	Phase("distribution")
	assert.Equal(t, 75, state.progress, "the distribution phase again after the plugins phase")

	Step("Waiting for a default StorageClass", 60, 120)
	assert.Equal(t, 87, state.progress)
}

//nolint:paralleltest // changes the package state.
func Test_KappProgressNeverGoesBack(t *testing.T) {
	var buf bytes.Buffer

	// The order of the lines of a lab run with --start-from distribution: two distribution runs.
	state.out, state.phases = &buf, []string{"distribution", "plugins"}
	t.Cleanup(func() { state.out, state.phases, state.phase, state.msg, state.progress = nil, nil, -1, "", noProgress })

	s := &Scanner{}

	for _, step := range []struct {
		line     string
		progress int
	}{
		{"", 0},
		{"1:00:00PM: ---- applying 9 changes [0/9 done] ----", 0},
		{"1:00:01PM: ---- waiting on 9 changes [0/9 done] ----", 0},
		{"1:00:02PM: ---- waiting on 3 changes [6/9 done] ----", 33},
		{"1:00:03PM: ---- applying complete [9/9 done] ----", 33},
		{"1:00:04PM: ---- waiting complete [9/9 done] ----", 50},
		{"", 50},
		{"1:00:05PM: ---- waiting on 38 changes [1/38 done] ----", 50},
	} {
		if step.line == "" {
			Phase("distribution")
		} else {
			_, err := s.Write([]byte(step.line + "\n"))
			assert.NoError(t, err)
		}

		assert.Equal(t, step.progress, state.progress, step.line)
	}
}
