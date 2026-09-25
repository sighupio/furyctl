// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package upgradeanalysis_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/upgradeanalysis"
)

func TestTextRendersTheChainAndEachHop(t *testing.T) {
	t.Parallel()

	fsys := hopsFS([]string{"1.34.0-1.34.1", "1.34.1-1.35.1"}, []string{"1.34.1-1.35.1"})

	unchanged := manifest{
		networking: "v3.1.0", ingress: "v5.0.0", monitoring: "v4.1.0", logging: "v5.3.0",
		tracing: "v1.4.0", opa: "v1.16.0", auth: "v0.6.1", dr: "v3.3.0",
		k8s: "1.34.4", installer: "v1.34.4",
	}

	patched := unchanged
	patched.ingress = "v5.0.1"

	fetcher := fakeFetcher{
		"v1.34.0": unchanged.kfd(),
		"v1.34.1": patched.kfd(),
		"v1.35.1": manifest{
			networking: "v4.0.0", ingress: "v5.1.0", monitoring: "v4.2.0", logging: "v5.4.0",
			tracing: "v1.5.0", opa: "v1.17.0", auth: "v0.7.0", dr: "v3.4.0",
			k8s: "1.35.5", installer: "v1.35.5",
		}.kfd(),
	}

	analysis, err := upgradeanalysis.Build(fsys, fetcher, qaCluster("v1.34.0"), nil, "v1.35.1")
	require.NoError(t, err, "Build")

	out := upgradeanalysis.Text(analysis)

	assert.Contains(t, out, "Upgrade path (2 hops): v1.34.0 -> v1.34.1 -> v1.35.1", "chain line")
	assert.Contains(t, out, "distribution only", "the cheap hop is called out")
	assert.Contains(t, out, "1.34.4 (unchanged)", "an unchanged Kubernetes version says so")
	assert.Contains(t, out, "1.34.4 -> 1.35.5", "a moving Kubernetes version shows both ends")
	assert.Contains(t, out, "Not deployed on this cluster, left out of the report: Tracing, Policy",
		"the omission is stated rather than silent")
	assert.Contains(t, out, "cilium", "the deployed type is shown next to the module")

	// Nothing about a module the cluster does not deploy should reach the output.
	assert.NotContains(t, out, "v1.17.0", "the policy module version must not leak into the report")
	assert.NotContains(t, out, "v1.5.0", "the tracing module version must not leak into the report")
}

func TestTextAlreadyAtTarget(t *testing.T) {
	t.Parallel()

	fsys := hopsFS([]string{"1.34.1-1.35.1"}, nil)

	analysis, err := upgradeanalysis.Build(fsys, fakeFetcher{}, qaCluster("v1.35.1"), nil, "v1.35.1")
	require.NoError(t, err, "Build")

	out := upgradeanalysis.Text(analysis)

	assert.Contains(t, out, "already runs the target version", "says there is nothing to do")
	assert.NotContains(t, out, "Upgrade path", "no chain to render")
	// Must not panic on the empty chain, which is the easy mistake here.
	assert.NotEmpty(t, strings.TrimSpace(out), "still renders the header")
}

func TestTextSingleHopWording(t *testing.T) {
	t.Parallel()

	fsys := hopsFS([]string{"1.34.1-1.35.1"}, []string{"1.34.1-1.35.1"})

	fetcher := fakeFetcher{
		"v1.34.1": manifest{
			networking: "v3.1.0", ingress: "v5.0.1", monitoring: "v4.1.0", logging: "v5.3.0",
			tracing: "v1.4.0", opa: "v1.16.0", auth: "v0.6.1", dr: "v3.3.0",
			k8s: "1.34.4", installer: "v1.34.4",
		}.kfd(),
		"v1.35.1": manifest{
			networking: "v4.0.0", ingress: "v5.1.0", monitoring: "v4.2.0", logging: "v5.4.0",
			tracing: "v1.5.0", opa: "v1.17.0", auth: "v0.7.0", dr: "v3.4.0",
			k8s: "1.35.5", installer: "v1.35.5",
		}.kfd(),
	}

	analysis, err := upgradeanalysis.Build(fsys, fetcher, qaCluster("v1.34.1"), nil, "v1.35.1")
	require.NoError(t, err, "Build")

	assert.Contains(t, upgradeanalysis.Text(analysis), "Upgrade path (1 hop):", "singular wording")
}

func TestTextStatesTheConfigurationCheckOutcome(t *testing.T) {
	t.Parallel()

	fsys := hopsFS([]string{"1.34.1-1.35.1"}, []string{"1.34.1-1.35.1"})

	fetcher := pathFetcher{
		from:    "v1.34.1",
		to:      "v1.35.1",
		fromDir: distDir(t, schemaBefore),
		toDir:   distDir(t, schemaAfter),
	}

	t.Run("a clean check says so rather than staying silent", func(t *testing.T) {
		t.Parallel()

		// A loki cluster: no customOutputs block, and kubeProxy never set.
		cfg := map[string]any{
			"spec": map[string]any{
				"distribution": map[string]any{
					"modules": map[string]any{"logging": map[string]any{"type": "loki"}},
				},
			},
		}

		analysis, err := upgradeanalysis.Build(fsys, fetcher, qaCluster("v1.34.1"), cfg, "v1.35.1")
		require.NoError(t, err, "Build")

		assert.True(t, analysis.ConfigChecked, "ConfigChecked")
		assert.Contains(t, upgradeanalysis.Text(analysis), "Configuration: checked, no changes required",
			"a clean result must be stated, not implied by silence")
	})

	t.Run("an unreadable configuration is reported as not checked", func(t *testing.T) {
		t.Parallel()

		analysis, err := upgradeanalysis.Build(fsys, fetcher, qaCluster("v1.34.1"), nil, "v1.35.1")
		require.NoError(t, err, "Build")

		assert.False(t, analysis.ConfigChecked, "ConfigChecked")
		assert.Contains(t, upgradeanalysis.Text(analysis), "Configuration: not checked",
			"the report must not imply a clean configuration it never looked at")
	})
}

// TestCleanConfigurationMessageIsTrustworthy is the regression guard for the message
// "Configuration: checked, no changes required". That sentence is a claim about the
// cluster, so every path that cannot substantiate it must say something else.
func TestCleanConfigurationMessageIsTrustworthy(t *testing.T) {
	t.Parallel()

	const cleanMessage = "Configuration: checked, no changes required"

	fsys := hopsFS([]string{"1.34.1-1.35.1"}, []string{"1.34.1-1.35.1"})

	// A cluster that sets kubeProxy.enabled, which the target schema no longer accepts.
	affected := map[string]any{
		"spec": map[string]any{
			"kubernetes": map[string]any{
				"advanced": map[string]any{"kubeProxy": map[string]any{"enabled": true}},
			},
		},
	}

	// A cluster on loki, which none of the schema changes can touch.
	unaffected := map[string]any{
		"spec": map[string]any{
			"distribution": map[string]any{
				"modules": map[string]any{"logging": map[string]any{"type": "loki"}},
			},
		},
	}

	t.Run("claimed only when the check ran and found nothing", func(t *testing.T) {
		t.Parallel()

		fetcher := pathFetcher{
			from:    "v1.34.1",
			to:      "v1.35.1",
			fromDir: distDir(t, schemaBefore),
			toDir:   distDir(t, schemaAfter),
		}

		analysis, err := upgradeanalysis.Build(fsys, fetcher, qaCluster("v1.34.1"), unaffected, "v1.35.1")
		require.NoError(t, err, "Build")

		require.Len(t, analysis.Hops, 1, "one hop")
		assert.Empty(t, analysis.Hops[0].ConfigCheckError, "the check must have run")
		assert.Contains(t, upgradeanalysis.Text(analysis), cleanMessage, "clean result")
	})

	t.Run("never claimed when the cluster is affected", func(t *testing.T) {
		t.Parallel()

		fetcher := pathFetcher{
			from:    "v1.34.1",
			to:      "v1.35.1",
			fromDir: distDir(t, schemaBefore),
			toDir:   distDir(t, schemaAfter),
		}

		analysis, err := upgradeanalysis.Build(fsys, fetcher, qaCluster("v1.34.1"), affected, "v1.35.1")
		require.NoError(t, err, "Build")

		out := upgradeanalysis.Text(analysis)
		require.NotEmpty(t, analysis.Hops[0].Findings, "the cluster is affected")
		assert.NotContains(t, out, cleanMessage, "a cluster with findings must never be called clean")
		assert.Contains(t, out, "kubeProxy.enabled", "the finding itself")
	})

	t.Run("never claimed when the schemas cannot be read", func(t *testing.T) {
		t.Parallel()

		// Distribution directories with no schema inside: the comparison cannot run. A
		// failed check yields no findings either, which is exactly how this used to be
		// misreported as a clean configuration.
		fetcher := pathFetcher{
			from:    "v1.34.1",
			to:      "v1.35.1",
			fromDir: t.TempDir(),
			toDir:   t.TempDir(),
		}

		analysis, err := upgradeanalysis.Build(fsys, fetcher, qaCluster("v1.34.1"), unaffected, "v1.35.1")
		require.NoError(t, err, "Build must still produce the version deltas")

		out := upgradeanalysis.Text(analysis)

		require.Empty(t, analysis.Hops[0].Findings, "a failed check produces no findings")
		assert.NotEmpty(t, analysis.Hops[0].ConfigCheckError, "the failure must be recorded on the hop")
		assert.NotContains(t, out, cleanMessage, "a failed check must never read as clean")
		assert.Contains(t, out, "could not be checked", "the failure must be visible in the hop")
		assert.NotEmpty(t, analysis.Warnings, "and also surfaced as a warning")
	})

	t.Run("never claimed when the configuration was unavailable", func(t *testing.T) {
		t.Parallel()

		fetcher := pathFetcher{
			from:    "v1.34.1",
			to:      "v1.35.1",
			fromDir: distDir(t, schemaBefore),
			toDir:   distDir(t, schemaAfter),
		}

		analysis, err := upgradeanalysis.Build(fsys, fetcher, qaCluster("v1.34.1"), nil, "v1.35.1")
		require.NoError(t, err, "Build")

		assert.NotContains(t, upgradeanalysis.Text(analysis), cleanMessage,
			"without a configuration there is nothing to call clean")
	})
}
