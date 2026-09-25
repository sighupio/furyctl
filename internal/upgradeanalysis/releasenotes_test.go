// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package upgradeanalysis_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/upgradeanalysis"
)

// notesDir writes release notes where a downloaded distribution keeps them.
func notesDir(t *testing.T, version, body string) string {
	t.Helper()

	dir := t.TempDir()
	releases := filepath.Join(dir, "docs", "releases")
	require.NoError(t, os.MkdirAll(releases, 0o755), "creating the releases directory")
	require.NoError(t,
		os.WriteFile(filepath.Join(releases, version+".md"), []byte(body), 0o600),
		"writing the notes",
	)

	return dir
}

// notesFetcher serves one hop, with release notes only for the target.
type notesFetcher struct {
	from, to string
	toDir    string
}

func (f notesFetcher) Fetch(_, version string) (upgradeanalysis.Distribution, error) {
	kfd := manifest{
		networking: "v3.1.0", ingress: "v5.0.1", monitoring: "v4.1.0", logging: "v5.3.0",
		tracing: "v1.4.0", opa: "v1.16.0", auth: "v0.6.1", dr: "v3.3.0",
		k8s: "1.34.4", installer: "v1.34.4",
	}.kfd()

	if version == f.to {
		return upgradeanalysis.Distribution{Manifest: kfd, Path: f.toDir}, nil
	}

	return upgradeanalysis.Distribution{Manifest: kfd, Path: t0Dir}, nil
}

// t0Dir is a directory with no release notes, standing in for the hop's source version,
// whose notes the analysis never reads.
var t0Dir = os.TempDir()

func hopWithNotes(t *testing.T, dir string) upgradeanalysis.Hop {
	t.Helper()

	fsys := hopsFS([]string{"1.34.1-1.35.1"}, []string{"1.34.1-1.35.1"})

	analysis, err := upgradeanalysis.Build(
		fsys,
		notesFetcher{from: "v1.34.1", to: "v1.35.1", toDir: dir},
		qaCluster("v1.34.1"),
		nil,
		"v1.35.1",
	)
	require.NoError(t, err, "Build")
	require.Len(t, analysis.Hops, 1, "one hop")

	return analysis.Hops[0]
}

func TestBreakingChangesAreTakenFromTheRelease(t *testing.T) {
	t.Parallel()

	// Both spellings occur across real releases, with an emoji after the heading.
	for _, heading := range []string{"## Breaking changes 💔", "## Breaking Changes 💔"} {
		t.Run(heading, func(t *testing.T) {
			t.Parallel()

			dir := notesDir(t, "v1.35.1", `# Release

## New features 🌟

- something new

`+heading+`

- cgroup v1 support removed: the kubelet will refuse to start.
- kubeProxy.enabled replaced by kubeProxy.type.

## Upgrade procedure

Check the docs.
`)

			hop := hopWithNotes(t, dir)

			assert.Empty(t, hop.BreakingChangesError, "the notes were readable")
			require.Len(t, hop.BreakingChanges, 1, "one release in this hop")

			section := hop.BreakingChanges[0].Section
			assert.Contains(t, section, "cgroup v1 support removed",
				"the release's own wording is carried through")
			assert.Contains(t, section, "kubeProxy.enabled", "every entry")
			assert.NotContains(t, section, "something new", "the preceding section must not leak in")
			assert.NotContains(t, section, "Check the docs", "the section ends at the next heading")
		})
	}
}

// A release that omits the section and a release that declares "None" are different facts,
// and neither may be reported as "no breaking changes".
func TestBreakingChangesDistinguishesAbsentFromNone(t *testing.T) {
	t.Parallel()

	t.Run("a release that omits the section", func(t *testing.T) {
		t.Parallel()

		hop := hopWithNotes(t, notesDir(t, "v1.35.1", "# Release\n\n## New features 🌟\n\n- something\n"))

		require.Len(t, hop.BreakingChanges, 1, "the release is still listed")
		assert.Empty(t, hop.BreakingChanges[0].Section, "nothing to show for it")
		assert.Empty(t, hop.BreakingChangesError, "that is not a failure")

		out := upgradeanalysis.Text(&upgradeanalysis.Analysis{Hops: []upgradeanalysis.Hop{hop}})
		assert.Contains(t, out, "list no breaking-changes section",
			"the absence of the section is stated, not turned into a clean claim")
	})

	t.Run("a release that declares None", func(t *testing.T) {
		t.Parallel()

		hop := hopWithNotes(t, notesDir(t, "v1.35.1", "# Release\n\n## Breaking changes 💔\n\nNone.\n"))

		require.Len(t, hop.BreakingChanges, 1, "the release is listed")
		assert.Equal(t, "None.", hop.BreakingChanges[0].Section,
			"the release's own answer is carried through")
	})
}

func TestBreakingChangesUnreadableNotes(t *testing.T) {
	t.Parallel()

	// A distribution directory with no docs at all.
	hop := hopWithNotes(t, t.TempDir())

	assert.NotEmpty(t, hop.BreakingChangesError, "the failure is recorded")
	assert.Empty(t, hop.BreakingChanges, "and nothing is invented")

	out := upgradeanalysis.Text(&upgradeanalysis.Analysis{Hops: []upgradeanalysis.Hop{hop}})
	assert.Contains(t, out, "could not be read", "a failed read is never reported as no changes")
	assert.NotContains(t, out, "list no breaking-changes section",
		"an unreadable file is not the same as a release without the section")
}

// TestBreakingChangesCoverSkippedReleases is the behaviour that matters most here: an upgrade
// path can jump over a release that is never installed, and everything that release declared
// breaking still applies to the cluster. Reading only the target's notes would hide it.
func TestBreakingChangesCoverSkippedReleases(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	releases := filepath.Join(dir, "docs", "releases")
	require.NoError(t, os.MkdirAll(releases, 0o755), "creating the releases directory")

	write := func(name, body string) {
		require.NoError(t, os.WriteFile(filepath.Join(releases, name), []byte(body), 0o600), name)
	}

	// The hop goes from v1.34.1 to v1.35.1, so v1.35.0 is never installed.
	write("v1.34.1.md", "# r\n\n## Breaking changes 💔\n\nalready applied, must not appear\n")
	write("v1.35.0.md", "# r\n\n## Breaking changes 💔\n\nskipped release, still applies\n")
	write("v1.35.1.md", "# r\n\n## Breaking changes 💔\n\nthe target release\n")

	hop := hopWithNotes(t, dir)

	require.Len(t, hop.BreakingChanges, 2, "the skipped release and the target, not the source")
	assert.Equal(t, "v1.35.0", hop.BreakingChanges[0].Version, "oldest first")
	assert.Equal(t, "skipped release, still applies", hop.BreakingChanges[0].Section,
		"a release the upgrade jumps over still declares breaking changes that apply")
	assert.Equal(t, "v1.35.1", hop.BreakingChanges[1].Version, "then the target")

	// The version the cluster already runs has nothing left to tell it.
	for _, release := range hop.BreakingChanges {
		assert.NotEqual(t, "v1.34.1", release.Version, "the source version is already applied")
	}
}
