// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package immutable //nolint:testpackage // exercises the unexported upgrade state initializer.

import (
	"testing"

	r3diff "github.com/r3labs/diff/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	commcreate "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/common/create"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/immutable/public"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/upgrade"
)

// The phases write their status without a nil test, so every phase an operation
// touches must exist here. The load balancer upgrade writes the three
// infrastructure phases (see create.Infrastructure.upgradeLoadBalancers).
func TestInitUpgradeStateCoversEveryPhase(t *testing.T) {
	t.Parallel()

	state := (&ClusterCreator{}).initUpgradeState()

	phases := map[string]*upgrade.Phase{
		"PreInfrastructure":  state.Phases.PreInfrastructure,
		"Infrastructure":     state.Phases.Infrastructure,
		"PostInfrastructure": state.Phases.PostInfrastructure,
		"PreKubernetes":      state.Phases.PreKubernetes,
		"Kubernetes":         state.Phases.Kubernetes,
		"PostKubernetes":     state.Phases.PostKubernetes,
		"PreDistribution":    state.Phases.PreDistribution,
		"Distribution":       state.Phases.Distribution,
		"PostDistribution":   state.Phases.PostDistribution,
	}

	for name, phase := range phases {
		require.NotNilf(t, phase, "phase %s is nil, a write to its status panics", name)
		assert.Equal(t, upgrade.PhaseStatusPending, phase.Status, "phase %s", name)
	}
}

// --upgrade-node routes on the role of the host: a worker goes to the kubernetes
// phase, a load balancer to the infrastructure phase, and the rest is rejected
// before any phase runs.
func TestUpgradeNodeRole(t *testing.T) {
	t.Parallel()

	conf := public.ImmutableKfdV1Alpha2{
		Spec: public.Spec{
			Infrastructure: public.SpecInfrastructure{
				LoadBalancers: &public.SpecInfrastructureLoadBalancers{
					Members: []public.Member{{Hostname: "lb01"}},
				},
			},
			Kubernetes: public.SpecKubernetes{
				ControlPlane: public.SpecKubernetesControlPlane{
					Members: []public.Member{{Hostname: "cp01"}},
				},
				Etcd: &public.SpecKubernetesEtcd{
					Members: []public.Member{{Hostname: "etcd01"}},
				},
				NodeGroups: []public.SpecKubernetesNodeGroup{
					{Name: "workers", Nodes: []public.Member{{Hostname: "worker01"}}},
				},
			},
		},
	}

	tests := []struct {
		name        string
		upgradeNode string
		wantRole    string
		errContains string
	}{
		{"not set", "", public.NodeRoleNone, ""},
		{"worker", "worker01", public.NodeRoleWorker, ""},
		{"load balancer", "lb01", public.NodeRoleLoadBalancer, ""},
		{"control plane", "cp01", "", "one at a time"},
		{"etcd", "etcd01", "", "one at a time"},
		{"unknown host", "nope01", "", "not a host of this cluster configuration"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			creator := &ClusterCreator{furyctlConf: conf, upgradeNode: test.upgradeNode}

			role, err := creator.upgradeNodeRole()

			if test.errContains != "" {
				require.Error(t, err)
				assert.ErrorIs(t, err, ErrUpgradeNodeUnsupported)
				assert.ErrorContains(t, err, test.errContains)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.wantRole, role)
		})
	}
}

// The role of the --upgrade-node host selects the phase that upgrades it, so a phase
// that the user selects as well runs the playbook of the other role, or no playbook.
// The rule belongs to this kind only: OnPremises upgrades every --upgrade-node host in
// its kubernetes phase, so a phase selection is valid there.
func TestValidateUpgradeNodeRejectsAPhaseSelection(t *testing.T) {
	t.Parallel()

	conf := public.ImmutableKfdV1Alpha2{
		Spec: public.Spec{
			Infrastructure: public.SpecInfrastructure{
				LoadBalancers: &public.SpecInfrastructureLoadBalancers{
					Members: []public.Member{{Hostname: "lb01"}},
				},
			},
			Kubernetes: public.SpecKubernetes{
				NodeGroups: []public.SpecKubernetesNodeGroup{
					{Name: "workers", Nodes: []public.Member{{Hostname: "worker01"}}},
				},
			},
		},
	}

	tests := []struct {
		name            string
		upgradeNode     string
		phase           string
		startFrom       string
		postApplyPhases []string
		wantErr         bool
	}{
		{"load balancer alone", "lb01", cluster.OperationPhaseAll, StartFromFlagNotSet, nil, false},
		{"worker alone", "worker01", cluster.OperationPhaseAll, StartFromFlagNotSet, nil, false},
		{"no upgrade node with a phase", "", cluster.OperationPhaseKubernetes, StartFromFlagNotSet, nil, false},
		{"no upgrade node with start from", "", cluster.OperationPhaseAll, cluster.OperationPhaseKubernetes, nil, false},
		{"no upgrade node with extra phases", "", cluster.OperationPhaseAll, StartFromFlagNotSet, []string{"distribution"}, false},
		{"load balancer with the wrong phase", "lb01", cluster.OperationPhaseKubernetes, StartFromFlagNotSet, nil, true},
		{"load balancer with its own phase", "lb01", cluster.OperationPhaseInfrastructure, StartFromFlagNotSet, nil, true},
		{"worker with a phase", "worker01", cluster.OperationPhaseKubernetes, StartFromFlagNotSet, nil, true},
		{"load balancer with start from", "lb01", cluster.OperationPhaseAll, cluster.OperationPhaseKubernetes, nil, true},
		{"worker with extra phases", "worker01", cluster.OperationPhaseAll, StartFromFlagNotSet, []string{"distribution"}, true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			creator := &ClusterCreator{
				furyctlConf:     conf,
				upgradeNode:     test.upgradeNode,
				phase:           test.phase,
				postApplyPhases: test.postApplyPhases,
			}

			err := creator.validateUpgradeNode(test.startFrom)

			if test.wantErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, ErrUpgradeNodeUnsupported)

				return
			}

			require.NoError(t, err)
		})
	}
}

// readUpgradeState reads a stored upgrade state twice, and each reading answers a
// different question. This test pins both answers, because one reading for both is what
// makes a resumed upgrade start again from the first phase.
func TestReadUpgradeState(t *testing.T) {
	t.Parallel()

	// The shape an older furyctl version stored: it failed in the distribution phase,
	// and it never tracked the infrastructure sub-phases.
	raw := []byte(`phases:
  infrastructure:
    status: success
  preKubernetes:
    status: success
  kubernetes:
    status: success
  postKubernetes:
    status: success
  preDistribution:
    status: success
  distribution:
    status: failed
`)

	creator := &ClusterCreator{upgradeStateStore: upgrade.NewStateStore("", "", "")}

	upgradeState, startFrom, err := creator.readUpgradeState(raw, StartFromFlagNotSet)
	require.NoError(t, err)

	// The resume continues from the phase that failed, and not from the first phase
	// of the order, although this version of furyctl tracks phases that the state
	// does not hold.
	assert.Equal(t, cluster.OperationPhaseDistribution, startFrom)

	// Every phase exists, so no write to one of them panics.
	phases := map[string]*upgrade.Phase{
		"PreInfrastructure":  upgradeState.Phases.PreInfrastructure,
		"Infrastructure":     upgradeState.Phases.Infrastructure,
		"PostInfrastructure": upgradeState.Phases.PostInfrastructure,
		"PreKubernetes":      upgradeState.Phases.PreKubernetes,
		"Kubernetes":         upgradeState.Phases.Kubernetes,
		"PostKubernetes":     upgradeState.Phases.PostKubernetes,
		"PreDistribution":    upgradeState.Phases.PreDistribution,
		"Distribution":       upgradeState.Phases.Distribution,
		"PostDistribution":   upgradeState.Phases.PostDistribution,
	}
	for name, phase := range phases {
		require.NotNilf(t, phase, "phase %s is absent, a write to its status panics", name)
	}

	// A stored status wins over the pending one, and a phase that the state does not
	// hold stays pending.
	assert.Equal(t, upgrade.PhaseStatusFailed, upgradeState.Phases.Distribution.Status)
	assert.Equal(t, upgrade.PhaseStatusSuccess, upgradeState.Phases.Infrastructure.Status)
	assert.Equal(t, upgrade.PhaseStatusPending, upgradeState.Phases.PreInfrastructure.Status)

	// A phase that the caller gives is kept.
	_, startFrom, err = creator.readUpgradeState(raw, cluster.OperationPhaseKubernetes)
	require.NoError(t, err)
	assert.Equal(t, cluster.OperationPhaseKubernetes, startFrom)
}

// workerNodes gives the hosts that the staged rollout upgrades one at a time. The
// Immutable configuration holds them under the node groups, and no other role list
// belongs in the result.
func TestWorkerNodes(t *testing.T) {
	t.Parallel()

	c := &ClusterCreator{
		furyctlConf: public.ImmutableKfdV1Alpha2{
			Spec: public.Spec{
				Infrastructure: public.SpecInfrastructure{
					LoadBalancers: &public.SpecInfrastructureLoadBalancers{
						Members: []public.Member{{Hostname: "lb01"}},
					},
				},
				Kubernetes: public.SpecKubernetes{
					ControlPlane: public.SpecKubernetesControlPlane{
						Members: []public.Member{{Hostname: "cp01"}},
					},
					Etcd: &public.SpecKubernetesEtcd{
						Members: []public.Member{{Hostname: "etcd01"}},
					},
					NodeGroups: []public.SpecKubernetesNodeGroup{
						{Name: "infra", Nodes: []public.Member{{Hostname: "infra01"}}},
						{Name: "workers", Nodes: []public.Member{
							{Hostname: "worker01"},
							{Hostname: "worker02"},
						}},
					},
				},
			},
		},
	}

	assert.Equal(t, []string{"infra01", "worker01", "worker02"}, c.workerNodes())
}

// A configuration with no node group gives an empty list, and not a nil one, so that the
// kubernetes phase stages nothing.
func TestWorkerNodesWithoutNodeGroups(t *testing.T) {
	t.Parallel()

	c := &ClusterCreator{}

	assert.Empty(t, c.workerNodes())
}

// Create() wraps the distribution phase in the upgrade decorator, which hides StorageSkipper.
// The creator must keep the undecorated phase, or the distribution phase never runs again.
func TestNewDistributionPhaseKeepsTheStorageSkipper(t *testing.T) {
	t.Parallel()

	c := &ClusterCreator{}
	phase := c.newDistributionPhase(upgrade.New(cluster.CreatorPaths{}, "Immutable"))

	require.NotNil(t, c.distribution)
	assert.Same(t, phase.Self(), c.distribution.Self(), "c.distribution must be the phase that the decorator wraps")

	var skipper commcreate.StorageSkipper = c.distribution
	assert.Empty(t, skipper.SkippedStoragePackages())

	_, ok := any(phase).(commcreate.StorageSkipper)
	assert.False(t, ok, "the decorator hides StorageSkipper, use c.distribution")
}

// A run for one host applies only its playbook, and then stores the configuration as applied.
// A change in the configuration is then lost, so the run refuses it.
func TestValidateUpgradeNodeChanges(t *testing.T) {
	t.Parallel()

	changes := r3diff.Changelog{{
		Type: "create",
		Path: []string{"spec", "distribution", "customPatches"},
		To:   map[string]any{},
	}}

	versionChange := r3diff.Changelog{{
		Type: "update",
		Path: []string{"spec", "distributionVersion"},
		From: "v1.35.1",
		To:   "v1.36.0",
	}}

	tests := []struct {
		name        string
		upgradeNode string
		changes     r3diff.Changelog
		wantErr     string
	}{
		{name: "no host, a change: apply as usual", changes: changes},
		{name: "a host, no change: upgrade the host", upgradeNode: "node1"},
		{name: "a host and a change: refuse", upgradeNode: "node1", changes: changes, wantErr: "run 'furyctl apply' to"},
		{name: "a load balancer and a change: refuse", upgradeNode: "lb1", changes: changes, wantErr: "run 'furyctl apply' to"},
		{
			// A plain apply refuses a version change, so the advice names the upgrade.
			name:        "a host and a version change: refuse, name the upgrade",
			upgradeNode: "node1",
			changes:     versionChange,
			wantErr:     "--upgrade --skip-nodes-upgrade",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := (&ClusterCreator{upgradeNode: tc.upgradeNode}).validateUpgradeNodeChanges(tc.changes)

			if tc.wantErr != "" {
				require.ErrorIs(t, err, errUpgradeNodeChanges)
				assert.ErrorContains(t, err, tc.wantErr)

				return
			}

			require.NoError(t, err)
		})
	}
}

type rawUpgradeStore struct {
	raw []byte
}

func (*rawUpgradeStore) Store(*upgrade.State) error { return nil }
func (*rawUpgradeStore) Delete() error              { return nil }
func (*rawUpgradeStore) GetLatestResumablePhase(*upgrade.State) string {
	return ""
}

func (s *rawUpgradeStore) Get() ([]byte, error) {
	if s.raw == nil {
		return nil, upgrade.ErrStateNotFound
	}

	return s.raw, nil
}

// routeStagedUpgrade sends a run with no staged worker to the phases. A run for one host must
// then find no change, with or without a stored upgrade state.
func TestRouteStagedUpgradeRefusesUpgradeNodeChanges(t *testing.T) {
	t.Parallel()

	changes := r3diff.Changelog{{
		Type: "create",
		Path: []string{"spec", "distribution", "customPatches"},
		To:   map[string]any{},
	}}

	tests := []struct {
		name string
		raw  []byte
	}{
		{name: "no upgrade state"},
		{name: "an upgrade state with no staged worker", raw: []byte("phases:\n  preKubernetes:\n    status: success\n")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			c := &ClusterCreator{
				furyctlConf:       labConf(),
				upgradeNode:       "node1",
				phase:             cluster.OperationPhaseAll,
				upgradeStateStore: &rawUpgradeStore{raw: tc.raw},
			}

			done, err := c.routeStagedUpgrade(nil, nil, changes, StartFromFlagNotSet)

			require.ErrorIs(t, err, errUpgradeNodeChanges)
			assert.False(t, done)
		})
	}
}
