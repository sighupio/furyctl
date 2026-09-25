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

	"github.com/sighupio/furyctl/internal/clusterhealth"
	"github.com/sighupio/furyctl/internal/upgradeanalysis"
)

func TestMarkdownProducesTheDocumentSkeleton(t *testing.T) {
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

	analysis.Health = &clusterhealth.Report{Checks: []clusterhealth.Check{
		{Name: "pods", Description: "pods that are not Running"},
		{Name: "node-usage", Description: "nodes close to saturation", Err: "metrics API not available"},
	}}

	out := upgradeanalysis.Markdown(analysis)

	// The section headings of the document it is meant to seed.
	assert.Contains(t, out, "# Upgrade Analysis — test-qa — SD v1.34.0 → v1.35.1", "title")
	assert.Contains(t, out, "# Cluster Inventory", "inventory section")
	assert.Contains(t, out, "# Cluster health Analysis", "health section")
	assert.Contains(t, out, "# Upgrade Plan", "plan section")

	assert.Contains(t, out, "| Intermediate versions | v1.34.1 |", "the intermediate hop is called out")
	assert.Contains(t, out, "| Not deployed | Tracing, Policy |", "omissions stay visible")
	assert.Contains(t, out, "## Hop v1.34.0 → v1.34.1", "first hop")
	assert.Contains(t, out, "## Hop v1.34.1 → v1.35.1", "second hop")
	assert.Contains(t, out, "Phase: distribution only", "the cheap hop is marked")
	assert.Contains(t, out, "Phase: Kubernetes and distribution",
		"and the phase is always stated, so its absence is never ambiguous")
	assert.Contains(t, out, "| Module | Type | Current version | New version | Update Notes |",
		"the module table matches the one the document uses, with a column left for prose")

	// The same honesty rules as the text renderer.
	assert.Contains(t, out, "node-usage** — could not be checked", "a failed check says so")
	assert.Contains(t, out, "Configuration: not checked", "no configuration means no clean claim")
	assert.NotContains(t, out, "Configuration: checked, no changes required",
		"a configuration that was never read must not be called clean")
}

func TestMarkdownAlreadyAtTarget(t *testing.T) {
	t.Parallel()

	fsys := hopsFS([]string{"1.34.1-1.35.1"}, nil)

	analysis, err := upgradeanalysis.Build(fsys, fakeFetcher{}, qaCluster("v1.35.1"), nil, "v1.35.1")
	require.NoError(t, err, "Build")

	out := upgradeanalysis.Markdown(analysis)

	assert.Contains(t, out, "already runs the target version", "says there is nothing to do")
	assert.NotContains(t, out, "# Upgrade Plan", "no plan to write")
	assert.NotEmpty(t, strings.TrimSpace(out), "the inventory is still rendered")
}
