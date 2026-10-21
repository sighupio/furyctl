// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package create

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

type erroringHelmfile struct {
	fakeHelmfile

	initErr error
	diffErr error
}

func (f *erroringHelmfile) Init(string) error { return f.initErr }

func (f *erroringHelmfile) Diff() (string, string, int, error) { return "", "", 0, f.diffErr }

type erroringShell struct {
	fakeShell

	err error
}

func (f *erroringShell) RunCapture(...string) (string, int, error) { return "", 0, f.err }

func TestPluginsDiff_HelmfileErrorFailsTheHelmSection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		helmfile *erroringHelmfile
		want     string
	}{
		{"init", &erroringHelmfile{initErr: errors.New("no repo")}, "Plugins (helm) Failed: helmfile init failed: no repo"},
		{"diff", &erroringHelmfile{diffErr: errors.New("no helmfile")}, "Plugins (helm) Failed: no helmfile"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := newPluginsSetup(t, "v1.36.0", true, nil, []string{"poc-podinfo"}, false)
			s.plugins.helmfileRunner = tc.helmfile

			sections, err := s.plugins.Diff()

			require.NoError(t, err)
			require.Equal(t, tc.want, describe(sections))
		})
	}
}

func TestPluginsDiff_InvalidReleaseBlockFailsThatRelease(t *testing.T) {
	t.Parallel()

	s := newPluginsSetup(t, "v1.36.0", true, nil, []string{"poc-podinfo"}, false)
	s.helmf.stdout = "Comparing release=poc-podinfo, chart=podinfo/podinfo\nPlan: 1 to add, 0 to change, 0 to destroy.\n"

	sections, err := s.plugins.Diff()

	require.NoError(t, err)
	require.Equal(t, "Plugin helm/poc-podinfo Failed: helmfile diff output did not match the expected format", describe(sections))
}

func TestPluginsDiff_KustomizeRunnerErrorFailsEveryKustomizePlugin(t *testing.T) {
	t.Parallel()

	s := newPluginsSetup(t, "v1.36.0", true, []string{"alpha", "beta"}, nil, false)
	s.plugins.shellRunner = &erroringShell{err: errors.New("no shell")}

	sections, err := s.plugins.Diff()

	require.NoError(t, err)
	require.Equal(t,
		"Plugin kustomize/alpha Failed: scripts/diff.sh could not run: no shell; "+
			"Plugin kustomize/beta Failed: scripts/diff.sh could not run: no shell",
		describe(sections))
}

func TestPluginsDiff_RenderErrorIsReturned(t *testing.T) {
	t.Parallel()

	s := newPluginsSetup(t, "v1.36.0", true, []string{"alpha"}, nil, false)
	require.NoError(t, os.Remove(s.plugins.paths.ConfigPath))

	sections, err := s.plugins.Diff()

	require.Error(t, err)
	require.Nil(t, sections)
}
