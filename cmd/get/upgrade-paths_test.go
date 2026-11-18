// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package get_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/cmd/get"
	"github.com/sighupio/furyctl/configs"
)

const upgradeGuideLine = "Upgrade guide: https://docs.sighup.io/docs/installation/upgrades/"

func upgradesFS(kind string, folders ...string) fstest.MapFS {
	fsys := fstest.MapFS{}
	for _, f := range folders {
		fsys["upgrades/"+kind+"/"+f+"/placeholder"] = &fstest.MapFile{}
	}

	return fsys
}

func releaseNotes(v string) string {
	return "https://github.com/sighupio/distribution/blob/v" + v + "/docs/releases/v" + v + ".md"
}

// Scenario: Three direct targets.
func TestFormatUpgradePaths_DirectTargetsTable(t *testing.T) {
	t.Parallel()

	fsys := upgradesFS("onpremises", "1.34.1-1.35.1", "1.34.1-1.34.2", "1.34.1-1.35.0")

	out, err := get.FormatUpgradePaths(fsys, "OnPremises", "1.34.1")
	require.NoError(t, err)

	assert.Contains(t, out, "Current version: 1.34.1 (OnPremises)")
	assert.Contains(t, out, "Direct upgrades:")
	assert.Regexp(t, `  TARGET\s+RELEASE NOTES\n`, out)
	assert.NotContains(t, out, "TYPE")
	assert.Regexp(t, regexp.MustCompile(
		`  1\.34\.2\s+`+regexp.QuoteMeta(releaseNotes("1.34.2"))+`\n`+
			`  1\.35\.0\s+`+regexp.QuoteMeta(releaseNotes("1.35.0"))+`\n`+
			`  1\.35\.1\s+`+regexp.QuoteMeta(releaseNotes("1.35.1"))+`\n`), out)
}

// Requirement: the output matches the agreed example of the brief literally (2-space indent).
func TestFormatUpgradePaths_BriefExample(t *testing.T) {
	t.Parallel()

	fsys := upgradesFS("onpremises", "1.34.1-1.34.2", "1.34.1-1.35.0", "1.34.1-1.35.1", "1.35.1-1.36.0")

	out, err := get.FormatUpgradePaths(fsys, "OnPremises", "1.34.1")
	require.NoError(t, err)

	assert.Equal(t, "Current version: 1.34.1 (OnPremises)\n"+
		"\n"+
		"Direct upgrades:\n"+
		"  TARGET   RELEASE NOTES\n"+
		"  1.34.2   "+releaseNotes("1.34.2")+"\n"+
		"  1.35.0   "+releaseNotes("1.35.0")+"\n"+
		"  1.35.1   "+releaseNotes("1.35.1")+"\n"+
		"\n"+
		"Path to the latest version (1.36.0), one upgrade at a time:\n"+
		"  1.34.1 -> 1.35.1 -> 1.36.0\n"+
		"  Read the release notes of 1.35.1 and 1.36.0 before you start.\n"+
		"\n"+
		"Next step: set spec.distributionVersion to v1.35.1 in furyctl.yaml and run furyctl apply --upgrade\n"+
		upgradeGuideLine+"\n", out)
}

// Requirement: Direct upgrades table, sorted by semantic version, never by string.
func TestFormatUpgradePaths_SemverOrder(t *testing.T) {
	t.Parallel()

	fsys := upgradesFS("onpremises", "1.8.0-1.10.0", "1.8.0-1.9.0")

	out, err := get.FormatUpgradePaths(fsys, "OnPremises", "1.8.0")
	require.NoError(t, err)

	assert.Regexp(t, `  1\.9\.0\s+\S+\n  1\.10\.0\s+\S+\n`, out)
	// Latest by semver is 1.10.0, a direct target: no path section.
	assert.NotContains(t, out, "Path to the latest version")
	assert.Contains(t, out, "Next step: set spec.distributionVersion to v1.10.0 in furyctl.yaml and run furyctl apply --upgrade")
}

// Scenarios: Multi-hop path, Next step uses the first hop.
func TestFormatUpgradePaths_MultiHop(t *testing.T) {
	t.Parallel()

	fsys := upgradesFS("onpremises",
		"1.34.1-1.34.2", "1.34.1-1.35.0", "1.34.1-1.35.1", "1.35.1-1.36.0", "1.34.2-1.35.0")

	out, err := get.FormatUpgradePaths(fsys, "OnPremises", "1.34.1")
	require.NoError(t, err)

	assert.Contains(t, out, "Path to the latest version (1.36.0), one upgrade at a time:")
	assert.Contains(t, out, "1.34.1 -> 1.35.1 -> 1.36.0\n")
	assert.Contains(t, out, "Read the release notes of 1.35.1 and 1.36.0 before you start.")
	assert.Contains(t, out, "Next step: set spec.distributionVersion to v1.35.1 in furyctl.yaml and run furyctl apply --upgrade\n"+
		upgradeGuideLine+"\n")
}

// Requirement: Path to the latest version, more than two hops.
func TestFormatUpgradePaths_ThreeHops(t *testing.T) {
	t.Parallel()

	fsys := upgradesFS("onpremises", "1.0.0-1.1.0", "1.1.0-1.2.0", "1.2.0-1.3.0")

	out, err := get.FormatUpgradePaths(fsys, "OnPremises", "1.0.0")
	require.NoError(t, err)

	assert.Contains(t, out, "1.0.0 -> 1.1.0 -> 1.2.0 -> 1.3.0\n")
	assert.Contains(t, out, "Read the release notes of 1.1.0, 1.2.0 and 1.3.0 before you start.")
	assert.Contains(t, out, "Next step: set spec.distributionVersion to v1.1.0 in furyctl.yaml")
}

// Requirement: Path to the latest version, the shortest chain wins over a longer one through higher hops.
func TestFormatUpgradePaths_ShortestChain(t *testing.T) {
	t.Parallel()

	fsys := upgradesFS("onpremises", "1.0.0-1.1.0", "1.0.0-1.2.0", "1.1.0-1.5.0", "1.2.0-1.3.0", "1.3.0-1.5.0")

	out, err := get.FormatUpgradePaths(fsys, "OnPremises", "1.0.0")
	require.NoError(t, err)

	assert.Contains(t, out, "1.0.0 -> 1.1.0 -> 1.5.0\n")
}

// Scenario: Latest is a direct upgrade.
func TestFormatUpgradePaths_LatestIsDirect(t *testing.T) {
	t.Parallel()

	fsys := upgradesFS("onpremises", "1.35.1-1.35.2", "1.35.1-1.36.0", "1.35.2-1.36.0")

	out, err := get.FormatUpgradePaths(fsys, "OnPremises", "1.35.1")
	require.NoError(t, err)

	assert.NotContains(t, out, "Path to the latest version")
	assert.NotContains(t, out, "Read the release notes of")
	assert.Contains(t, out, "Next step: set spec.distributionVersion to v1.36.0 in furyctl.yaml and run furyctl apply --upgrade")
	assert.Contains(t, out, upgradeGuideLine)
}

// Scenario: Tie between shortest chains.
func TestFormatUpgradePaths_TiePicksHighestHop(t *testing.T) {
	t.Parallel()

	fsys := upgradesFS("onpremises", "1.0.0-1.1.0", "1.0.0-1.2.0", "1.1.0-1.3.0", "1.2.0-1.3.0")

	out, err := get.FormatUpgradePaths(fsys, "OnPremises", "1.0.0")
	require.NoError(t, err)

	assert.Contains(t, out, "1.0.0 -> 1.2.0 -> 1.3.0\n")
}

// Requirement: Path to the latest version, when every reachable version is lower than the current one the
// highest of them is the latest and the chain still has a first hop.
func TestFormatUpgradePaths_OnlyLowerTargets(t *testing.T) {
	t.Parallel()

	fsys := upgradesFS("onpremises", "1.5.0-1.4.0")

	out, err := get.FormatUpgradePaths(fsys, "OnPremises", "1.5.0")
	require.NoError(t, err)

	assert.NotContains(t, out, "Path to the latest version")
	assert.Contains(t, out, "Next step: set spec.distributionVersion to v1.4.0 in furyctl.yaml")
}

// Scenario: No folder for the version.
func TestFormatUpgradePaths_NoPaths(t *testing.T) {
	t.Parallel()

	fsys := upgradesFS("onpremises", "1.35.1-1.36.0")

	out, err := get.FormatUpgradePaths(fsys, "OnPremises", "1.36.0")
	require.NoError(t, err)

	assert.Equal(t, "No upgrade paths: 1.36.0 is the latest version furyctl can upgrade to.\n", out)
}

// Scenario: No folder for the version, kind without any upgrades folder.
func TestFormatUpgradePaths_NoKindFolder(t *testing.T) {
	t.Parallel()

	out, err := get.FormatUpgradePaths(fstest.MapFS{}, "Immutable", "1.0.0")
	require.NoError(t, err)

	assert.Equal(t, "No upgrade paths: 1.0.0 is the latest version furyctl can upgrade to.\n", out)
}

// Scenario: Real embed check.
func TestUpgradePathsEmbedFoldersAreStableVersions(t *testing.T) {
	t.Parallel()

	stable := regexp.MustCompile(`^\d+\.\d+\.\d+-\d+\.\d+\.\d+$`)

	folders, err := fs.Glob(configs.Tpl, "upgrades/*/*")
	require.NoError(t, err)
	require.NotEmpty(t, folders)

	for _, f := range folders {
		info, err := fs.Stat(configs.Tpl, f)
		require.NoError(t, err)

		if info.IsDir() {
			assert.Regexp(t, stable, info.Name(), f)
		}
	}
}

func writeFuryctlYaml(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "furyctl.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	return path
}

// Scenario: Both values from the file.
func TestReadUpgradePathsConfig_BothValues(t *testing.T) {
	t.Parallel()

	path := writeFuryctlYaml(t, "apiVersion: kfd.sighup.io/v1alpha2\nkind: OnPremises\nmetadata:\n  name: test\nspec:\n  distributionVersion: v1.34.2\n")

	kind, version, err := get.ReadUpgradePathsConfig(path)
	require.NoError(t, err)

	assert.Equal(t, "OnPremises", kind)
	assert.Equal(t, "v1.34.2", version)
}

// Scenario: Missing file.
func TestReadUpgradePathsConfig_MissingFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "furyctl.yaml")

	_, _, err := get.ReadUpgradePathsConfig(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), path)
}

// Scenario: Empty field.
func TestReadUpgradePathsConfig_EmptyField(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		field   string
	}{
		{name: "no distributionVersion", content: "kind: OnPremises\nspec: {}\n", field: "spec.distributionVersion"},
		{name: "no kind", content: "spec:\n  distributionVersion: v1.34.2\n", field: "kind"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := get.ReadUpgradePathsConfig(writeFuryctlYaml(t, tc.content))
			require.ErrorIs(t, err, get.ErrUpgradePathsConfigField)
			assert.Contains(t, err.Error(), tc.field)
		})
	}
}
