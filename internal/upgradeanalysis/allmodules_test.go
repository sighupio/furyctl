// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package upgradeanalysis_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/apis/config"
	"github.com/sighupio/furyctl/internal/clusterinfo"
	"github.com/sighupio/furyctl/internal/upgradeanalysis"
)

// fullCluster deploys every SD module. The other fixtures switch tracing and policy off, so
// without this the module mapping is only ever exercised with two modules missing, and a
// module wired to the wrong kfd.yaml field would go unnoticed.
func fullCluster(version string) *clusterinfo.Info {
	return &clusterinfo.Info{
		ClusterName: "full",
		SDKind:      "OnPremises",
		SDVersion:   version,
		Modules: []clusterinfo.ModuleInfo{
			{Name: clusterinfo.ModuleNetworking, Type: "calico"},
			{Name: clusterinfo.ModuleIngress, Type: "nginx/single"},
			{Name: clusterinfo.ModuleMonitoring, Type: "mimir"},
			{Name: clusterinfo.ModuleLogging, Type: "opensearch"},
			{Name: clusterinfo.ModuleTracing, Type: "tempo"},
			{Name: clusterinfo.ModulePolicy, Type: "gatekeeper"},
			{Name: clusterinfo.ModuleAuth, Type: "sso"},
			{Name: clusterinfo.ModuleDR, Type: "on-premises"},
		},
	}
}

func TestEveryModuleIsReportedWhenAllAreDeployed(t *testing.T) {
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

	analysis, err := upgradeanalysis.Build(fsys, fetcher, fullCluster("v1.34.1"), nil, "v1.35.1")
	require.NoError(t, err, "Build")
	require.Len(t, analysis.Hops, 1, "one hop")

	assert.Empty(t, analysis.Skipped, "nothing is switched off on this cluster")

	deltas := map[string]upgradeanalysis.ModuleDelta{}
	for _, delta := range analysis.Hops[0].Modules {
		deltas[delta.Name] = delta
	}

	// Every module must be present, and carry the versions of the kfd.yaml field it maps to.
	// Policy is the one worth pinning: its display name and its manifest field ("opa") differ.
	want := map[string][2]string{
		clusterinfo.ModuleNetworking: {"v3.1.0", "v4.0.0"},
		clusterinfo.ModuleIngress:    {"v5.0.1", "v5.1.0"},
		clusterinfo.ModuleMonitoring: {"v4.1.0", "v4.2.0"},
		clusterinfo.ModuleLogging:    {"v5.3.0", "v5.4.0"},
		clusterinfo.ModuleTracing:    {"v1.4.0", "v1.5.0"},
		clusterinfo.ModulePolicy:     {"v1.16.0", "v1.17.0"},
		clusterinfo.ModuleAuth:       {"v0.6.1", "v0.7.0"},
		clusterinfo.ModuleDR:         {"v3.3.0", "v3.4.0"},
	}

	require.Len(t, deltas, len(want), "every deployed module is reported")

	for name, versions := range want {
		delta, reported := deltas[name]
		require.True(t, reported, "%s is deployed and must be reported", name)
		assert.Equal(t, versions[0], delta.From, "%s from", name)
		assert.Equal(t, versions[1], delta.To, "%s to", name)
		assert.NotEmpty(t, delta.Type, "%s must carry the deployed type", name)
	}

	assert.Len(t, analysis.Hops[0].ChangedModules(), len(want), "all eight move in this hop")
	assert.Empty(t, analysis.Hops[0].UnchangedModules(), "none stays put")
}

// TestAWSModuleOnEKS covers the one module that only exists for a single kind.
func TestAWSModuleOnEKS(t *testing.T) {
	t.Parallel()

	fsys := fsWithKind("ekscluster", "1.34.1-1.35.1")

	withAWS := func(version, aws string) config.KFD {
		kfd := manifest{
			networking: "v3.1.0", ingress: "v5.0.1", monitoring: "v4.1.0", logging: "v5.3.0",
			tracing: "v1.4.0", opa: "v1.16.0", auth: "v0.6.1", dr: "v3.3.0",
		}.kfd()
		kfd.Modules.Aws = aws
		kfd.Kubernetes.Eks = config.KFDProvider{Version: version, Installer: "v3.5.0"}

		return kfd
	}

	fetcher := fakeFetcher{
		"v1.34.1": withAWS("1.34", "v5.2.0"),
		"v1.35.1": withAWS("1.35", "v5.3.0"),
	}

	info := fullCluster("v1.34.1")
	info.SDKind = "EKSCluster"
	info.Modules = append(info.Modules, clusterinfo.ModuleInfo{Name: clusterinfo.ModuleAWS})

	analysis, err := upgradeanalysis.Build(fsys, fetcher, info, nil, "v1.35.1")
	require.NoError(t, err, "Build")
	require.Len(t, analysis.Hops, 1, "one hop")

	var aws *upgradeanalysis.ModuleDelta

	for i, delta := range analysis.Hops[0].Modules {
		if delta.Name == clusterinfo.ModuleAWS {
			aws = &analysis.Hops[0].Modules[i]
		}
	}

	// The AWS module reports no type, so it must not be mistaken for a module that is
	// switched off.
	require.NotNil(t, aws, "the AWS module is deployed on EKS and must be reported")
	assert.Equal(t, "v5.2.0", aws.From, "aws from")
	assert.Equal(t, "v5.3.0", aws.To, "aws to")
	assert.NotContains(t, analysis.Skipped, clusterinfo.ModuleAWS, "an empty type is not 'not deployed'")

	assert.Equal(t, "1.34", analysis.Hops[0].KubernetesFrom, "EKS reads its own Kubernetes field")
	assert.Equal(t, "1.35", analysis.Hops[0].KubernetesTo, "EKS reads its own Kubernetes field")
}
