// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package create

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/upgrade"
	"github.com/sighupio/furyctl/pkg/reducers"
)

var errFake = errors.New("fake error")

const (
	noResources = "No resources found"
	noDefault   = "NAME         PROVISIONER\nlocal-path   rancher.io/local-path"
	withDefault = "NAME                 PROVISIONER          RECLAIMPOLICY\n" +
		"longhorn (default)   driver.longhorn.io   Delete\n" +
		"longhorn-static      driver.longhorn.io   Delete"
)

// fakeLister returns the outputs in order, and repeats the last one.
type fakeLister struct {
	outs  []string
	errs  []error
	calls int
}

func (f *fakeLister) Get(_ bool, _ string, _ ...string) (string, error) {
	i := min(f.calls, len(f.outs)-1)
	f.calls++

	var err error
	if i < len(f.errs) {
		err = f.errs[i]
	}

	return f.outs[i], err
}

func modulesData(m map[any]any) map[string]map[any]any {
	return map[string]map[any]any{"spec": {"distribution": map[any]any{"modules": m}}}
}

func typed(t string) map[any]any { return map[any]any{"type": t} }

func TestDefaultStorageClassName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		out  string
		want string
	}{
		{name: "no storage classes", out: noResources, want: ""},
		{name: "no default", out: noDefault, want: ""},
		{name: "one default among several", out: withDefault, want: "longhorn"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, defaultStorageClassName(tc.out))
		})
	}
}

func TestSkippedStoragePackages(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data map[string]map[any]any
		want []string
	}{
		{name: "no modules", data: map[string]map[any]any{}, want: nil},
		{
			name: "all storage modules disabled",
			data: modulesData(map[any]any{
				"dr": typed("none"), "logging": typed("none"),
				"tracing": typed("none"), "monitoring": typed("none"),
			}),
			want: nil,
		},
		{
			name: "prometheus agent needs no storage",
			data: modulesData(map[any]any{"monitoring": typed("prometheusAgent")}),
			want: nil,
		},
		{name: "logging", data: modulesData(map[any]any{"logging": typed("loki")}), want: []string{"logging"}},
		{name: "dr", data: modulesData(map[any]any{"dr": typed("on-premises")}), want: []string{"dr"}},
		{name: "tempo", data: modulesData(map[any]any{"tracing": typed("tempo")}), want: []string{"tracing"}},
		{
			name: "prometheus",
			data: modulesData(map[any]any{"monitoring": typed("prometheus")}),
			want: []string{"prometheus-operated"},
		},
		{name: "mimir", data: modulesData(map[any]any{"monitoring": typed("mimir")}), want: []string{"mimir"}},
		{
			name: "all of them",
			data: modulesData(map[any]any{
				"dr": typed("on-premises"), "logging": typed("opensearch"),
				"tracing": typed("tempo"), "monitoring": typed("prometheus"),
			}),
			want: []string{"dr", "logging", "tracing", "prometheus-operated"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, skippedStoragePackages(tc.data))
		})
	}
}

func TestCheckDefaultStorageClass(t *testing.T) {
	t.Parallel()

	logging := modulesData(map[any]any{"logging": typed("loki")})

	tests := []struct {
		name          string
		out           string
		err           error
		data          map[string]map[any]any
		wantAvailable bool
		wantSkipped   []string
		wantErr       bool
	}{
		{name: "default StorageClass", out: withDefault, data: logging, wantAvailable: true},
		{name: "no StorageClass", out: noResources, data: logging, wantSkipped: []string{"logging"}},
		{name: "no default StorageClass", out: noDefault, data: logging, wantSkipped: []string{"logging"}},
		{name: "no storage-backed package", out: noResources, data: modulesData(map[any]any{})},
		{name: "kubectl error", out: "", err: errFake, data: logging, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			check := NewStorageClassCheck(&fakeLister{outs: []string{tc.out}, errs: []error{tc.err}}, false)

			available, err := check.CheckDefaultStorageClass(tc.data)
			if tc.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantAvailable, available)
			assert.Equal(t, tc.wantSkipped, check.SkippedStoragePackages())
		})
	}
}

// A second render with the StorageClass available forgets the skipped packages, so the
// distribution phase runs again one time at most.
func TestCheckDefaultStorageClassClearsSkipped(t *testing.T) {
	t.Parallel()

	check := NewStorageClassCheck(&fakeLister{outs: []string{noResources, withDefault}}, false)
	data := modulesData(map[any]any{"dr": typed("on-premises")})

	_, err := check.CheckDefaultStorageClass(data)
	require.NoError(t, err)
	require.Equal(t, []string{"dr"}, check.SkippedStoragePackages())

	_, err = check.CheckDefaultStorageClass(data)
	require.NoError(t, err)
	assert.Empty(t, check.SkippedStoragePackages())
}

// After a wait finds the StorageClass, the next check uses it without asking the cluster, and the
// check after that asks the cluster again.
func TestCheckDefaultStorageClassUsesTheWaitResult(t *testing.T) {
	t.Parallel()

	lister := &fakeLister{outs: []string{withDefault, noResources}}
	check := NewStorageClassCheck(lister, false)
	check.skipped = []string{"dr"}
	data := modulesData(map[any]any{"dr": typed("on-premises")})

	name, found := check.WaitForDefaultStorageClass(50*time.Millisecond, time.Millisecond)
	require.True(t, found)
	require.Equal(t, "longhorn", name)
	require.Equal(t, 1, lister.calls)

	available, err := check.CheckDefaultStorageClass(data)
	require.NoError(t, err)
	assert.True(t, available)
	assert.Empty(t, check.SkippedStoragePackages())
	assert.Equal(t, 1, lister.calls, "the check after a successful wait does not call kubectl")

	available, err = check.CheckDefaultStorageClass(data)
	require.NoError(t, err)
	assert.False(t, available)
	assert.Equal(t, 2, lister.calls, "the next check asks the cluster again")
}

func TestWaitForDefaultStorageClass(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		lister    *fakeLister
		dryRun    bool
		wantName  string
		wantFound bool
	}{
		{
			name:      "appears after some polls",
			lister:    &fakeLister{outs: []string{noResources, noDefault, withDefault}},
			wantName:  "longhorn",
			wantFound: true,
		},
		{
			name:      "kubectl errors are retried",
			lister:    &fakeLister{outs: []string{"", withDefault}, errs: []error{errFake}},
			wantName:  "longhorn",
			wantFound: true,
		},
		{name: "never appears", lister: &fakeLister{outs: []string{noDefault}}},
		{name: "dry-run does not wait", lister: &fakeLister{outs: []string{withDefault}}, dryRun: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			name, found := NewStorageClassCheck(tc.lister, tc.dryRun).WaitForDefaultStorageClass(
				50*time.Millisecond,
				time.Millisecond,
			)

			assert.Equal(t, tc.wantName, name)
			assert.Equal(t, tc.wantFound, found)
		})
	}
}

type fakeSkipper struct {
	skipped  []string
	name     string
	found    bool
	waited   bool
	timeout  time.Duration
	reported int
}

func (f *fakeSkipper) SkippedStoragePackages() []string { return f.skipped }

func (f *fakeSkipper) ReportStillSkipped() bool {
	f.reported++

	return f.reported == 1
}

func (f *fakeSkipper) WaitForDefaultStorageClass(timeout, _ time.Duration) (string, bool) {
	f.waited = true
	f.timeout = timeout

	return f.name, f.found
}

// fakePhase shares the *upgrade.Upgrade with the creator, as the real phases do.
type fakePhase struct {
	upgr          *upgrade.Upgrade
	err           error
	execs         int
	rdcs          reducers.Reducers
	upgradeAtExec bool
	startFrom     string
	state         *upgrade.State
}

func (f *fakePhase) Exec(rdcs reducers.Reducers, startFrom string, state *upgrade.State) error {
	f.execs++
	f.rdcs = rdcs
	f.upgradeAtExec = f.upgr.Enabled
	f.startFrom = startFrom
	f.state = state

	return f.err
}

func (f *fakePhase) SetUpgrade(enabled bool) { f.upgr.Enabled = enabled }

func (*fakePhase) Self() *cluster.OperationPhase { return nil }

func TestReapplyDistribution(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		skipper    *fakeSkipper
		upgrade    bool
		execErr    error
		wantWaited bool
		wantExecs  int
		wantErr    bool
	}{
		{name: "nothing skipped", skipper: &fakeSkipper{found: true, name: "sc"}},
		{
			name:       "no StorageClass appears",
			skipper:    &fakeSkipper{skipped: []string{"logging"}},
			wantWaited: true,
		},
		{
			name:       "StorageClass appears",
			skipper:    &fakeSkipper{skipped: []string{"logging"}, found: true, name: "sc"},
			wantWaited: true,
			wantExecs:  1,
		},
		{
			name:       "StorageClass appears during an upgrade",
			skipper:    &fakeSkipper{skipped: []string{"logging"}, found: true, name: "sc"},
			upgrade:    true,
			wantWaited: true,
			wantExecs:  1,
		},
		{
			name:       "second run fails",
			skipper:    &fakeSkipper{skipped: []string{"logging"}, found: true, name: "sc"},
			upgrade:    true,
			execErr:    errFake,
			wantWaited: true,
			wantExecs:  1,
			wantErr:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			upgr := &upgrade.Upgrade{Enabled: tc.upgrade}
			phase := &fakePhase{upgr: upgr, err: tc.execErr, startFrom: "unset"}
			state := &upgrade.State{}

			err := ReapplyDistribution(phase, tc.skipper, time.Minute, upgr, state)
			if tc.wantErr {
				require.ErrorIs(t, err, errFake)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tc.wantWaited, tc.skipper.waited)

			if tc.wantWaited {
				assert.Equal(t, time.Minute, tc.skipper.timeout, "the wait of the caller must reach the poll")
			}
			assert.Equal(t, tc.wantExecs, phase.execs)
			assert.False(t, phase.upgradeAtExec, "the second run must not run the upgrade scripts")
			assert.Nil(t, phase.rdcs, "the second run must not run the reducers")
			assert.Equal(t, tc.upgrade, upgr.Enabled, "the upgrade flag must be restored")
			assert.Equal(t, &upgrade.State{}, state, "the second run must not change the upgrade state")

			if tc.wantExecs > 0 {
				assert.Empty(t, phase.startFrom, "the second run applies the whole phase")
				assert.Same(t, state, phase.state)
			}

			if tc.wantWaited && tc.wantExecs == 0 {
				assert.Equal(t, 1, tc.skipper.reported, "a StorageClass that does not appear is reported")
			}
		})
	}
}

// A wait of 0 checks the cluster one time, and does not log the wait.
func TestWaitForDefaultStorageClassZeroTimeout(t *testing.T) {
	t.Parallel()

	lister := &fakeLister{outs: []string{noResources, withDefault}}

	name, found := NewStorageClassCheck(lister, false).WaitForDefaultStorageClass(0, time.Millisecond)

	assert.False(t, found)
	assert.Empty(t, name)
	assert.Equal(t, 1, lister.calls)
}

type fakePlugins struct{ applied bool }

func (f fakePlugins) Applied() bool { return f.applied }

func TestPluginsStorageClassWait(t *testing.T) {
	t.Parallel()

	assert.Equal(t, StorageClassWaitTimeout, PluginsStorageClassWait(fakePlugins{applied: true}))
	assert.Zero(t, PluginsStorageClassWait(fakePlugins{applied: false}))
}

// The two check points of an apply log the skipped packages at info level one time. A new check
// that skips packages logs them again.
func TestReportStillSkipped(t *testing.T) {
	t.Parallel()

	check := NewStorageClassCheck(&fakeLister{outs: []string{noResources}}, false)
	data := modulesData(map[any]any{"dr": typed("on-premises")})

	_, err := check.CheckDefaultStorageClass(data)
	require.NoError(t, err)

	assert.True(t, check.ReportStillSkipped(), "first report at info level")
	assert.False(t, check.ReportStillSkipped(), "second report at debug level")

	_, err = check.CheckDefaultStorageClass(data)
	require.NoError(t, err)

	assert.True(t, check.ReportStillSkipped(), "a new check reports again")
}
