// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package cmd //nolint:testpackage // exercises the unexported report builder.

import (
	"testing"

	"github.com/stretchr/testify/require"

	commcreate "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/common/create"
)

func TestResourceLines_ClusterScopedFirstEvenWhenANamespacedAddressSortsBefore(t *testing.T) {
	t.Parallel()

	lines := resourceLines(commcreate.Section{Rows: []commcreate.Row{
		{Namespace: "Alpha", Kind: "ConfigMap", Name: "a", Op: opCreate},
		{Namespace: clusterScoped, Kind: "Namespace", Name: "demo", Op: opCreate},
	}})

	require.Equal(t, "+ Namespace/demo", lines[0].text)
	require.Equal(t, "+ Alpha/ConfigMap/a", lines[1].text)
}

func TestGroupPluginSections_DifferentStatusesWithTheSameReasonAreNotSummarized(t *testing.T) {
	t.Parallel()

	grouped := groupPluginSections([]commcreate.Section{
		{Title: "Plugin kustomize/a", Status: commcreate.StatusCompared},
		{Title: "Plugin kustomize/b", Status: commcreate.StatusFailed},
	})

	require.Len(t, grouped, 2)
	require.Equal(t, "Plugin kustomize/a", grouped[0].Title)
	require.Equal(t, "Plugin kustomize/b", grouped[1].Title)
}

// A bidi override (U+202E) or a zero-width space (U+200B) in a name, a kind, a title, a reason or a line of the
// configuration is dropped, as the CONFIG table drops it, so that it cannot reorder or hide part of a line. A
// control character still becomes ?.
func TestBuildDiffReport_DropsFormatCharacters(t *testing.T) {
	t.Parallel()

	const (
		rlo  = "\u202e"
		zwsp = "\u200b"
	)

	report := buildDiffReport(
		configurationResult{Lines: []string{".spec.a" + rlo + ": 1 -> 2" + zwsp}},
		[]commcreate.Section{
			{
				Title: "Distribution" + zwsp, Status: commcreate.StatusCompared, Target: "Target cluster" + rlo,
				Rows: []commcreate.Row{
					{Namespace: "kube-system", Kind: "Config" + zwsp + "Map", Name: "evil" + rlo + "gpj.exe", Op: opUpdate},
					{Namespace: clusterScoped, Kind: "Namespace", Name: "demo" + zwsp, Op: opDelete},
					{Namespace: "kube-system", Kind: "Secret", Name: "esc\x1b[2Jx", Op: opDelete},
				},
			},
			{Title: "Plugin kustomize/alpha", Status: commcreate.StatusFailed, Reason: "kapp: Error:" + rlo + " boom" + zwsp},
		},
	)

	require.NotContains(t, report.Text, rlo)
	require.NotContains(t, report.Text, zwsp)
	require.NotContains(t, report.Text, "\x1b")
	require.Contains(t, report.Text, "Target cluster\n")
	require.Contains(t, report.Text, "Configuration\n  .spec.a: 1 -> 2\n")
	require.Contains(t, report.Text, "Distribution\n  - Namespace/demo\n  ~ kube-system/ConfigMap/evilgpj.exe\n  - kube-system/Secret/esc?[2Jx\n")
	require.Contains(t, report.Text, "Plugins (kustomize): not compared: kapp: Error: boom\n")
	require.Contains(t, report.Text, "Warning: Namespace demo will be deleted with everything in it.")
}

func TestPrintable(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"plain":            "kube-system/ConfigMap/a",
		"bidi override":    "a\u202eb",
		"zero-width space": "a\u200bb",
		"bidi isolate":     "a\u2066b\u2069",
		"byte order mark":  "\ufeffab",
	}

	want := map[string]string{
		"plain":            "kube-system/ConfigMap/a",
		"bidi override":    "ab",
		"zero-width space": "ab",
		"bidi isolate":     "ab",
		"byte order mark":  "ab",
	}

	for name, in := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, want[name], printable(in))
		})
	}

	require.Equal(t, "a?b?c?d", printable("a\x1bb\tc\nd"))
}
