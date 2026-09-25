// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package upgradeanalysis_test

import (
	"errors"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/apis/config"
	"github.com/sighupio/furyctl/internal/clusterinfo"
	"github.com/sighupio/furyctl/internal/upgradeanalysis"
)

// manifest describes a KFD fixture without the noise of a full struct literal.
type manifest struct {
	networking, ingress, monitoring, logging, tracing, opa, auth, dr string
	k8s, installer                                                   string
}

func (m manifest) kfd() config.KFD {
	return config.KFD{
		Modules: config.KFDModules{
			Networking: m.networking,
			Ingress:    m.ingress,
			Monitoring: m.monitoring,
			Logging:    m.logging,
			Tracing:    m.tracing,
			Opa:        m.opa,
			Auth:       m.auth,
			Dr:         m.dr,
		},
		Kubernetes: config.KFDKubernetes{
			OnPremises: config.KFDProvider{Version: m.k8s, Installer: m.installer},
		},
	}
}

type fakeFetcher map[string]config.KFD

func (f fakeFetcher) Manifest(_, version string) (config.KFD, error) {
	kfd, ok := f[version]
	if !ok {
		return config.KFD{}, errors.New("no fixture for " + version)
	}

	return kfd, nil
}

// hopsFS builds an upgrade-paths filesystem. A hop listed in withKubernetes also gets the
// pre-kubernetes script furyctl ships for hops that touch the Kubernetes phase.
func hopsFS(hops []string, withKubernetes []string) fstest.MapFS {
	fsys := fstest.MapFS{}

	for _, hop := range hops {
		base := "upgrades/onpremises/" + hop
		fsys[base+"/pre-distribution.sh.tpl"] = &fstest.MapFile{Data: []byte("#!/bin/sh\n")}

		for _, k8sHop := range withKubernetes {
			if k8sHop == hop {
				fsys[base+"/pre-kubernetes.sh.tpl"] = &fstest.MapFile{Data: []byte("#!/bin/sh\n")}
			}
		}
	}

	return fsys
}

// qaCluster mirrors the module set of a real OnPremises cluster: cilium, loki, prometheus,
// sso, on-premises DR, with tracing and policy switched off.
func qaCluster(version string) *clusterinfo.Info {
	return &clusterinfo.Info{
		ClusterName: "test-qa",
		SDKind:      "OnPremises",
		SDVersion:   version,
		Modules: []clusterinfo.ModuleInfo{
			{Name: clusterinfo.ModuleNetworking, Type: "cilium"},
			{Name: clusterinfo.ModuleIngress, Type: "nginx/dual"},
			{Name: clusterinfo.ModuleMonitoring, Type: "prometheus"},
			{Name: clusterinfo.ModuleLogging, Type: "loki"},
			{Name: clusterinfo.ModuleTracing, Type: "none"},
			{Name: clusterinfo.ModulePolicy, Type: "none"},
			{Name: clusterinfo.ModuleAuth, Type: "sso"},
			{Name: clusterinfo.ModuleDR, Type: "on-premises"},
		},
	}
}

func TestBuildResolvesTheWholeChain(t *testing.T) {
	t.Parallel()

	fsys := hopsFS(
		[]string{"1.32.0-1.33.1", "1.33.1-1.34.1", "1.34.1-1.35.1"},
		[]string{"1.32.0-1.33.1", "1.33.1-1.34.1", "1.34.1-1.35.1"},
	)

	fetcher := fakeFetcher{
		"v1.32.0": manifest{
			networking: "v2.2.0", ingress: "v4.0.0", monitoring: "v3.5.0", logging: "v5.1.0",
			tracing: "v1.2.0", opa: "v1.14.0", auth: "v0.5.1", dr: "v3.1.0",
			k8s: "1.32.4", installer: "v1.32.4",
		}.kfd(),
		"v1.33.1": manifest{
			networking: "v3.0.0", ingress: "v4.1.1", monitoring: "v4.0.1", logging: "v5.2.0",
			tracing: "v1.3.0", opa: "v1.15.0", auth: "v0.6.0", dr: "v3.2.0",
			k8s: "1.33.4", installer: "v1.33.4-rev.1",
		}.kfd(),
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

	analysis, err := upgradeanalysis.Build(fsys, fetcher, qaCluster("v1.32.0"), "v1.35.1")
	require.NoError(t, err, "Build")

	require.Len(t, analysis.Hops, 3, "three minors means three hops")
	assert.False(t, analysis.AlreadyAtTarget(), "AlreadyAtTarget")
	assert.Equal(t, "v1.32.0", analysis.From, "from")
	assert.Equal(t, "v1.35.1", analysis.To, "to")

	first := analysis.Hops[0]
	assert.Equal(t, "v1.32.0", first.From, "first hop from")
	assert.Equal(t, "v1.33.1", first.To, "first hop to")
	assert.Equal(t, "1.32.4", first.KubernetesFrom, "kubernetes from")
	assert.Equal(t, "1.33.4", first.KubernetesTo, "kubernetes to")
	assert.Equal(t, "v1.32.4", first.InstallerFrom, "installer from")
	assert.Equal(t, "v1.33.4-rev.1", first.InstallerTo, "installer to")
	assert.True(t, first.KubernetesChanged(), "kubernetes moves in the first hop")
	assert.False(t, first.DistributionOnly, "a hop with a pre-kubernetes script is not distribution only")

	last := analysis.Hops[2]
	assert.Equal(t, "v1.35.1", last.To, "last hop ends at the target")
	assert.Equal(t, "1.35.5", last.KubernetesTo, "last hop kubernetes")
}

func TestBuildReportsOnlyDeployedModules(t *testing.T) {
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

	analysis, err := upgradeanalysis.Build(fsys, fetcher, qaCluster("v1.34.1"), "v1.35.1")
	require.NoError(t, err, "Build")
	require.Len(t, analysis.Hops, 1, "one hop")

	names := make([]string, 0, len(analysis.Hops[0].Modules))
	for _, module := range analysis.Hops[0].Modules {
		names = append(names, module.Name)
	}

	// Tracing and Policy are switched off on this cluster and must not be reported, even
	// though their versions move in the manifests.
	assert.NotContains(t, names, clusterinfo.ModuleTracing, "tracing is not deployed")
	assert.NotContains(t, names, clusterinfo.ModulePolicy, "policy is not deployed")
	assert.Contains(t, names, clusterinfo.ModuleNetworking, "networking is deployed")
	assert.ElementsMatch(t,
		[]string{clusterinfo.ModuleTracing, clusterinfo.ModulePolicy},
		analysis.Skipped,
		"skipped modules are reported so the omission is visible",
	)

	// The deployed type travels with the delta, so a report can say "cilium", not just a version.
	for _, module := range analysis.Hops[0].Modules {
		if module.Name == clusterinfo.ModuleNetworking {
			assert.Equal(t, "cilium", module.Type, "networking type")
			assert.Equal(t, "v3.1.0", module.From, "networking from")
			assert.Equal(t, "v4.0.0", module.To, "networking to")
		}
	}
}

func TestBuildDistributionOnlyHop(t *testing.T) {
	t.Parallel()

	// v1.34.0 -> v1.34.1 ships no pre-kubernetes script and does not move Kubernetes.
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

	analysis, err := upgradeanalysis.Build(fsys, fetcher, qaCluster("v1.34.0"), "v1.35.1")
	require.NoError(t, err, "Build")
	require.Len(t, analysis.Hops, 2, "v1.34.0 cannot reach v1.35.1 directly")

	first := analysis.Hops[0]
	assert.True(t, first.DistributionOnly, "no pre-kubernetes script and no version move")
	assert.False(t, first.KubernetesChanged(), "kubernetes stays put")

	// Only ingress moves in that hop; everything else must be reported as unchanged.
	changed := first.ChangedModules()
	require.Len(t, changed, 1, "one module moves")
	assert.Equal(t, clusterinfo.ModuleIngress, changed[0].Name, "the module that moves")
	assert.Equal(t, "v5.0.0", changed[0].From, "ingress from")
	assert.Equal(t, "v5.0.1", changed[0].To, "ingress to")
	assert.NotEmpty(t, first.UnchangedModules(), "the rest are unchanged")

	assert.False(t, analysis.Hops[1].DistributionOnly, "the second hop does touch Kubernetes")
}

func TestBuildAlreadyAtTarget(t *testing.T) {
	t.Parallel()

	fsys := hopsFS([]string{"1.34.1-1.35.1"}, nil)

	analysis, err := upgradeanalysis.Build(fsys, fakeFetcher{}, qaCluster("v1.35.1"), "v1.35.1")
	require.NoError(t, err, "Build")

	assert.True(t, analysis.AlreadyAtTarget(), "AlreadyAtTarget")
	assert.Empty(t, analysis.Hops, "no hops")
	// No manifest is fetched at all in this case, which is why the fetcher above is empty.
}

func TestBuildErrors(t *testing.T) {
	t.Parallel()

	fsys := hopsFS([]string{"1.34.1-1.35.1"}, nil)

	t.Run("an unreachable target fails before anything is downloaded", func(t *testing.T) {
		t.Parallel()

		_, err := upgradeanalysis.Build(fsys, fakeFetcher{}, qaCluster("v1.34.1"), "v1.99.0")
		require.Error(t, err, "Build")
		assert.Contains(t, err.Error(), "cannot plan the upgrade", "error context")
	})

	t.Run("a missing manifest is reported with the version that is missing", func(t *testing.T) {
		t.Parallel()

		_, err := upgradeanalysis.Build(fsys, fakeFetcher{}, qaCluster("v1.34.1"), "v1.35.1")
		require.ErrorIs(t, err, upgradeanalysis.ErrNoManifest, "error kind")
		assert.Contains(t, err.Error(), "v1.34.1", "the missing version must be named")
	})
}
