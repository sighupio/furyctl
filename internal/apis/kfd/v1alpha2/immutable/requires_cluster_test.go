// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package immutable //nolint:testpackage // exercises the unexported preflight gate.

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/immutable/public"
	"github.com/sighupio/furyctl/internal/cluster"
)

// The distribution and the plugins phases read the cluster. The phases that build the
// cluster do not, so a run that starts with one of them continues without a cluster.
func TestRequiresCluster(t *testing.T) {
	t.Parallel()

	tests := []struct {
		phase string
		want  bool
	}{
		{cluster.OperationPhaseAll, false},
		{cluster.OperationPhaseInfrastructure, false},
		{cluster.OperationSubPhasePreInfrastructure, false},
		{cluster.OperationSubPhasePostInfrastructure, false},
		{cluster.OperationPhaseKubernetes, false},
		{cluster.OperationSubPhasePreKubernetes, false},
		{cluster.OperationSubPhasePostKubernetes, false},
		{cluster.OperationPhaseDistribution, true},
		{cluster.OperationSubPhasePreDistribution, true},
		{cluster.OperationSubPhasePostDistribution, true},
		{cluster.OperationPhasePlugins, true},
	}

	for _, test := range tests {
		t.Run(test.phase, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.want, requiresCluster(test.phase))
		})
	}
}

// --phase selects the first phase of the run. Without it the run starts where
// --start-from says, and an empty value starts at the infrastructure phase.
func TestFirstPhase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		phase     string
		startFrom string
		want      string
	}{
		{"phase selected", cluster.OperationPhaseDistribution, StartFromFlagNotSet, cluster.OperationPhaseDistribution},
		{"phase wins over start-from", cluster.OperationPhaseKubernetes, cluster.OperationPhasePlugins, cluster.OperationPhaseKubernetes},
		{"start-from alone", cluster.OperationPhaseAll, cluster.OperationSubPhasePreDistribution, cluster.OperationSubPhasePreDistribution},
		{"neither", cluster.OperationPhaseAll, StartFromFlagNotSet, cluster.OperationPhaseAll},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			c := &ClusterCreator{phase: test.phase}

			assert.Equal(t, test.want, c.firstPhase(test.startFrom))
		})
	}
}

// The message of the gate names the hosts that the preflight check probes, and its
// inventory holds the control plane group only.
func TestControlPlaneNodes(t *testing.T) {
	t.Parallel()

	c := &ClusterCreator{furyctlConf: public.ImmutableKfdV1Alpha2{
		Spec: public.Spec{
			Infrastructure: public.SpecInfrastructure{
				LoadBalancers: &public.SpecInfrastructureLoadBalancers{
					Members: []public.Member{{Hostname: "lb01"}},
				},
			},
			Kubernetes: public.SpecKubernetes{
				ControlPlane: public.SpecKubernetesControlPlane{
					Members: []public.Member{{Hostname: "cp01"}, {Hostname: "cp02"}},
				},
				Etcd: &public.SpecKubernetesEtcd{
					Members: []public.Member{{Hostname: "etcd01"}},
				},
				NodeGroups: []public.SpecKubernetesNodeGroup{
					{Name: "workers", Nodes: []public.Member{{Hostname: "worker01"}}},
				},
			},
		},
	}}

	assert.Equal(t, []string{"cp01", "cp02"}, c.controlPlaneNodes())
	assert.Equal(t, []string{"worker01"}, c.workerNodes())
}
