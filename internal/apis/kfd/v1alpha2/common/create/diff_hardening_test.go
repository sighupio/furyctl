// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package create_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/common/create"
)

func TestDiffDistribution_RunnerErrorFailsTheSection(t *testing.T) {
	t.Parallel()

	distro := newDistroSource(t, "v1.36.0", true, "distribution")
	phase, manifests := newPhaseFolder(t)

	section := create.DiffDistribution(distro, phase, &fakeRunner{dir: manifests, err: errors.New("no shell")})

	require.Equal(t, create.StatusFailed, section.Status)
	require.Equal(t, "scripts/diff.sh could not run: no shell", section.Reason)
}

func TestDiffDistribution_StaleOutputThatCannotBeRemovedFailsTheSection(t *testing.T) {
	t.Parallel()

	distro := newDistroSource(t, "v1.36.0", true, "distribution")
	phase, manifests := newPhaseFolder(t)

	stale := filepath.Join(manifests, "kapp-diff.json")
	require.NoError(t, os.MkdirAll(filepath.Join(stale, "child"), os.FileMode(0o755)))

	runner := &fakeRunner{dir: manifests}

	section := create.DiffDistribution(distro, phase, runner)

	require.Equal(t, create.StatusFailed, section.Status)
	require.Contains(t, section.Reason, "error removing earlier kapp output: ")
	require.Empty(t, runner.calls)
}

func TestDiffDistribution_NotSupportedVersionText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		version string
		reason  string
	}{
		{"version without v", "1.35.1", "distribution v1.35.1"},
		{"version with v", "v1.35.1", "distribution v1.35.1"},
		{"empty version", "", "distribution (unknown version)"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			distro := newDistroSource(t, tc.version, false, "distribution")
			phase, manifests := newPhaseFolder(t)

			section := create.DiffDistribution(distro, phase, &fakeRunner{dir: manifests})

			require.Equal(t, create.StatusNotSupported, section.Status)
			require.Equal(t, tc.reason, section.Reason)
		})
	}
}

func TestDiffDistribution_MissingKfdManifestIsAnUnknownVersion(t *testing.T) {
	t.Parallel()

	phase, manifests := newPhaseFolder(t)

	section := create.DiffDistribution(t.TempDir(), phase, &fakeRunner{dir: manifests})

	require.Equal(t, "distribution (unknown version)", section.Reason)
}
