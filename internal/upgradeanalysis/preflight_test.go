// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package upgradeanalysis_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/clusterhealth"
	"github.com/sighupio/furyctl/internal/clusterinfo"
	"github.com/sighupio/furyctl/internal/upgrade"
	"github.com/sighupio/furyctl/internal/upgradeanalysis"
)

func preflight(t *testing.T, info *clusterinfo.Info, name string) clusterhealth.Check {
	t.Helper()

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

	analysis, err := upgradeanalysis.Build(fsys, fetcher, info, nil, "v1.35.1")
	require.NoError(t, err, "Build")

	for _, check := range analysis.Preflight {
		if check.Name == name {
			return check
		}
	}

	t.Fatalf("no %q check in the preflight", name)

	return clusterhealth.Check{}
}

func TestOngoingUpgradeIsABlocker(t *testing.T) {
	t.Parallel()

	t.Run("an upgrade in flight blocks planning another", func(t *testing.T) {
		t.Parallel()

		info := qaCluster("v1.34.1")
		info.SDOngoingUpgrade = &clusterinfo.OngoingUpgrade{
			Status: string(upgrade.PhaseStatusFailed),
			Phase:  "distribution",
			From:   "v1.34.0",
			To:     "v1.34.1",
		}

		check := preflight(t, info, "ongoing-upgrade")

		require.Len(t, check.Issues, 1, "one blocker")
		assert.Equal(t, clusterhealth.SeverityBlocker, check.Issues[0].Severity, "severity")
		assert.Contains(t, check.Issues[0].Detail, "distribution", "the phase")
		assert.Contains(t, check.Issues[0].Detail, "finish or roll back", "what to do about it")
	})

	t.Run("no upgrade in flight is a clean check", func(t *testing.T) {
		t.Parallel()

		check := preflight(t, qaCluster("v1.34.1"), "ongoing-upgrade")

		assert.True(t, check.Clean(), "clean")
	})

	// The whole point of recording the lookup error: an unreadable upgrade state must not
	// read as "no upgrade in progress".
	t.Run("an unreadable upgrade state is not a clean check", func(t *testing.T) {
		t.Parallel()

		info := qaCluster("v1.34.1")
		info.SDOngoingUpgradeError = "configmaps is forbidden"

		check := preflight(t, info, "ongoing-upgrade")

		assert.False(t, check.Ran(), "the check could not run")
		assert.False(t, check.Clean(), "and must never be reported as clean")
		assert.Contains(t, check.Err, "forbidden", "the reason is kept")
	})
}
