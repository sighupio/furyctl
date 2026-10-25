// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package helmfile_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/tool/helmfile"
)

const (
	reasonFormat    = "helmfile diff output did not match the expected format"
	reasonOwnership = "helm-diff reports an ownership change"
	reasonDuplicate = "helmfile diff printed this release more than once"
	reasonMissing   = "helmfile diff printed no result for this release"
)

func readDiffFixture(t *testing.T, name string) string {
	t.Helper()

	b, err := os.ReadFile(filepath.Join("testdata", "diff", name))
	require.NoError(t, err)

	return string(b)
}

func countChanges(rel helmfile.ReleaseDiff, change helmfile.ChangeKind) int {
	n := 0

	for _, c := range rel.Changes {
		if c.Change == change {
			n++
		}
	}

	return n
}

func findRelease(t *testing.T, res helmfile.HelmDiffResult, name string) helmfile.ReleaseDiff {
	t.Helper()

	for _, r := range res.Releases {
		if r.Name == name {
			return r
		}
	}

	t.Fatalf("release %s not in the result", name)

	return helmfile.ReleaseDiff{}
}

func TestParseHelmDiff_Counts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		fixture string
		release string
		c, u, d int
	}{
		{"helmfile-diff-simple-new-releases.stdout", "poc-podinfo", 2, 0, 0},
		{"helmfile-diff-simple-new-releases.stdout", "poc-metrics", 9, 0, 0},
		{"helmfile-diff-simple-installed-hpa-removed.stdout", "poc-podinfo", 0, 1, 1},
		{"helmfile-diff-simple-podinfo-nochange.stdout", "poc-podinfo", 0, 0, 0},
		{"helmfile-diff-simple-podinfo-nochange.stdout", "poc-metrics", 9, 0, 0},
		{"synthetic-old-format.stdout", "poc-podinfo", 0, 1, 1},
		{"synthetic-suppressed.stdout", "poc-podinfo", 0, 2, 0},
		{"synthetic-comma-name.stdout", "poc-podinfo", 1, 0, 0},
	}

	for _, tc := range tests {
		t.Run(tc.fixture+"/"+tc.release, func(t *testing.T) {
			t.Parallel()

			res := helmfile.ParseHelmDiff(readDiffFixture(t, tc.fixture), "", 0, []string{"poc-podinfo", "poc-metrics"})

			rel := findRelease(t, res, tc.release)

			require.False(t, rel.NotCompared, rel.Reason)
			require.Equal(t, tc.c, countChanges(rel, helmfile.ChangeCreate))
			require.Equal(t, tc.u, countChanges(rel, helmfile.ChangeUpdate))
			require.Equal(t, tc.d, countChanges(rel, helmfile.ChangeDelete))
		})
	}
}

func TestParseHelmDiff_Rows(t *testing.T) {
	t.Parallel()

	tests := []struct {
		fixture string
		release string
		want    helmfile.ResourceChange
	}{
		{
			"helmfile-diff-simple-new-releases.stdout", "poc-metrics",
			helmfile.ResourceChange{Namespace: "poc-helm", Name: "system:metrics-server-aggregated-reader", Kind: "ClusterRole", API: "rbac.authorization.k8s.io", Change: helmfile.ChangeCreate},
		},
		{
			"helmfile-diff-simple-new-releases.stdout", "poc-metrics",
			helmfile.ResourceChange{Namespace: "poc-helm", Name: "poc-metrics-metrics-server:system:auth-delegator", Kind: "ClusterRoleBinding", API: "rbac.authorization.k8s.io", Change: helmfile.ChangeCreate},
		},
		{
			"helmfile-diff-simple-installed-hpa-removed.stdout", "poc-podinfo",
			helmfile.ResourceChange{Namespace: "poc-helm", Name: "poc-podinfo", Kind: "HorizontalPodAutoscaler", API: "autoscaling", Change: helmfile.ChangeDelete},
		},
		{
			"synthetic-suppressed.stdout", "poc-podinfo",
			helmfile.ResourceChange{Namespace: "poc-helm", Name: "poc-podinfo-config", Kind: "Secret", API: "v1", Change: helmfile.ChangeUpdate},
		},
		{
			"synthetic-comma-name.stdout", "poc-podinfo",
			helmfile.ResourceChange{Namespace: "poc-helm", Name: "poc-podinfo, extra", Kind: "ConfigMap", API: "v1", Change: helmfile.ChangeCreate},
		},
	}

	for _, tc := range tests {
		t.Run(tc.fixture+"/"+tc.want.Name, func(t *testing.T) {
			t.Parallel()

			res := helmfile.ParseHelmDiff(readDiffFixture(t, tc.fixture), "", 0, []string{"poc-podinfo", "poc-metrics"})

			require.Contains(t, findRelease(t, res, tc.release).Changes, tc.want)
		})
	}
}

func TestParseHelmDiff_SectionNotFailedWhenAllValid(t *testing.T) {
	t.Parallel()

	stdout := readDiffFixture(t, "helmfile-diff-simple-new-releases.stdout")

	res := helmfile.ParseHelmDiff(stdout, "", 0, []string{"poc-podinfo", "poc-metrics"})

	require.False(t, res.Failed())
	require.Empty(t, res.Reason)
	require.Len(t, res.Releases, 2)
}

func TestParseHelmDiff_NotCompared(t *testing.T) {
	t.Parallel()

	tests := []struct {
		fixture string
		reason  string
	}{
		{"synthetic-plan-mismatch.stdout", reasonFormat},
		{"synthetic-no-plan.stdout", reasonFormat},
		{"synthetic-unknown-line.stdout", reasonFormat},
		{"synthetic-old-destroy-mismatch.stdout", reasonFormat},
		{"synthetic-plan-first.stdout", reasonFormat},
		{"synthetic-ownership.stdout", reasonOwnership},
		{"synthetic-duplicate.stdout", reasonDuplicate},
	}

	for _, tc := range tests {
		t.Run(tc.fixture, func(t *testing.T) {
			t.Parallel()

			res := helmfile.ParseHelmDiff(readDiffFixture(t, tc.fixture), "", 0, []string{"poc-podinfo"})

			rel := findRelease(t, res, "poc-podinfo")

			require.True(t, rel.NotCompared)
			require.Equal(t, tc.reason, rel.Reason)
			require.Empty(t, rel.Changes)
			require.True(t, res.Failed())
		})
	}
}

func TestParseHelmDiff_MissingReleaseKeepsOthers(t *testing.T) {
	t.Parallel()

	stdout := readDiffFixture(t, "helmfile-diff-simple-podinfo-nochange.stdout")

	res := helmfile.ParseHelmDiff(stdout, "", 0, []string{"poc-podinfo", "poc-other"})

	other := findRelease(t, res, "poc-other")

	require.True(t, other.NotCompared)
	require.Equal(t, reasonMissing, other.Reason)
	require.False(t, findRelease(t, res, "poc-podinfo").NotCompared)
	require.True(t, res.Failed())
}

func TestParseHelmDiff_InvalidReleaseKeepsValidOne(t *testing.T) {
	t.Parallel()

	stdout := strings.Join([]string{
		"Comparing release=poc-podinfo, chart=podinfo/podinfo, namespace=poc-helm",
		"poc-helm, poc-podinfo, Deployment (apps) to be added.",
		"Plan: 2 to add, 0 to change, 0 to destroy, 0 to change ownership.",
		"",
		"Comparing release=poc-metrics, chart=metrics-server/metrics-server, namespace=poc-helm",
		"poc-helm, poc-metrics-metrics-server, Service (v1) to be added.",
		"Plan: 1 to add, 0 to change, 0 to destroy, 0 to change ownership.",
		"",
	}, "\n")

	res := helmfile.ParseHelmDiff(stdout, "", 0, []string{"poc-podinfo", "poc-metrics"})

	require.True(t, findRelease(t, res, "poc-podinfo").NotCompared)
	require.Equal(t, 1, countChanges(findRelease(t, res, "poc-metrics"), helmfile.ChangeCreate))
	require.True(t, res.Failed())
}

func TestParseHelmDiff_NonZeroExit(t *testing.T) {
	t.Parallel()

	unreachable := readDiffFixture(t, "helmfile-diff-simple-unreachable.stderr")

	tests := []struct {
		name   string
		stdout string
		stderr string
		code   int
		reason string
	}{
		{
			"unreachable cluster",
			readDiffFixture(t, "helmfile-diff-simple-unreachable.stdout"),
			unreachable,
			1,
			`helmfile diff exited 1: Error: Failed to get release poc-podinfo in namespace poc-helm: exit status 1: ` +
				`Error: Kubernetes cluster unreachable: Get "https://192.168.200.250:6999/version": ` +
				`dial tcp 192.168.200.250:6999: connect: connection refused`,
		},
		{
			"last non empty line",
			readDiffFixture(t, "helmfile-diff-simple-podinfo-nochange.stdout"),
			readDiffFixture(t, "synthetic-render-failure.stderr"),
			2,
			"helmfile diff exited 2: in ./helmfile.yaml: failed to render",
		},
		{
			"error not at the start of the line",
			"",
			readDiffFixture(t, "synthetic-last-line.stderr"),
			1,
			"helmfile diff exited 1: last failure",
		},
		{
			"first error line without indentation",
			"",
			readDiffFixture(t, "synthetic-first-error.stderr"),
			1,
			"helmfile diff exited 1: Error: first error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			res := helmfile.ParseHelmDiff(tc.stdout, tc.stderr, tc.code, []string{"poc-podinfo", "poc-metrics"})

			require.Equal(t, tc.reason, res.Reason)
			require.Empty(t, res.Releases)
			require.True(t, res.Failed())
		})
	}
}

func TestParseHelmDiff_NoReleases(t *testing.T) {
	t.Parallel()

	res := helmfile.ParseHelmDiff("", "", 0, nil)

	require.Empty(t, res.Releases)
	require.False(t, res.Failed())
}

func TestParseHelmDiff_OldFormatSuppressedOnly(t *testing.T) {
	t.Parallel()

	stdout := "Comparing release=poc-podinfo, chart=podinfo/podinfo\n" +
		"poc-helm, poc-podinfo-config, Secret (v1) has changed, but diff is empty after suppression.\n" +
		"Plan: 0 to add, 0 to change, 0 to destroy.\n"

	res := helmfile.ParseHelmDiff(stdout, "", 0, []string{"poc-podinfo"})

	rel := findRelease(t, res, "poc-podinfo")

	require.False(t, rel.NotCompared, rel.Reason)
	require.Equal(t, 1, countChanges(rel, helmfile.ChangeUpdate))
	require.False(t, res.Failed())
}

func TestParseHelmDiff_PlanCountBeyondTheIntegerRangeIsNotCompared(t *testing.T) {
	t.Parallel()

	out := "Comparing release=poc, chart=podinfo/podinfo, namespace=poc\n" +
		"Plan: 99999999999999999999999 to add, 0 to change, 0 to destroy, 0 to change ownership.\n"

	res := helmfile.ParseHelmDiff(out, "", 0, []string{"poc"})

	require.Equal(t, reasonFormat, findRelease(t, res, "poc").Reason)
}
