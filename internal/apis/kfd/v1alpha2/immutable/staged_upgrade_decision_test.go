// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package immutable //nolint:testpackage // exercises the unexported staged upgrade router.

import (
	"testing"

	r3diff "github.com/r3labs/diff/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/immutable/public"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/upgrade"
)

func stagedState(ready bool, nodes map[string]upgrade.PhaseStatus) *upgrade.State {
	return &upgrade.State{
		Phases:     succeededPhases(),
		Transition: &upgrade.Transition{From: "v1.35.1", To: "v1.36.0"},
		StagedWorkers: &upgrade.StagedWorkers{
			Nodes:          nodes,
			ReadyForResume: ready,
		},
	}
}

func labConf() public.ImmutableKfdV1Alpha2 {
	return public.ImmutableKfdV1Alpha2{
		Spec: public.Spec{
			Infrastructure: public.SpecInfrastructure{
				LoadBalancers: &public.SpecInfrastructureLoadBalancers{
					Members: []public.Member{{Hostname: "lb1"}},
				},
			},
			Kubernetes: public.SpecKubernetes{
				ControlPlane: public.SpecKubernetesControlPlane{
					Members: []public.Member{{Hostname: "cp1"}},
				},
				NodeGroups: []public.SpecKubernetesNodeGroup{
					{Name: "workers", Nodes: []public.Member{{Hostname: "node1"}, {Hostname: "node2"}}},
				},
			},
		},
	}
}

// stagedUpgradeDecision routes a run that finds pending workers. A wrong route either
// upgrades nothing or records a version that the workers do not run.
func TestStagedUpgradeDecision(t *testing.T) {
	t.Parallel()

	pending := map[string]upgrade.PhaseStatus{
		"node1": upgrade.PhaseStatusPending,
		"node2": upgrade.PhaseStatusPending,
	}

	tests := []struct {
		name             string
		state            *upgrade.State
		upgradeFlag      bool
		upgradeNode      string
		skipNodesUpgrade bool
		phase            string
		changes          r3diff.Changelog
		want             stagedUpgradeAction
		wantErr          bool
	}{
		{
			name:  "no state: run the phases",
			state: nil,
			want:  stagedUpgradeProceed,
		},
		{
			name:        "pending workers and no flag: refuse",
			state:       stagedState(true, pending),
			upgradeFlag: false,
			wantErr:     true,
		},
		{
			name:        "ready to resume: upgrade every pending worker",
			state:       stagedState(true, pending),
			upgradeFlag: true,
			phase:       cluster.OperationPhaseAll,
			want:        stagedUpgradeResumeBatch,
		},
		{
			name:             "skip the nodes again: report and stop",
			state:            stagedState(true, pending),
			upgradeFlag:      true,
			skipNodesUpgrade: true,
			phase:            cluster.OperationPhaseAll,
			want:             stagedUpgradeNoop,
		},
		{
			name:        "not ready yet and every phase succeeded: finalize",
			state:       stagedState(false, pending),
			upgradeFlag: true,
			phase:       cluster.OperationPhaseAll,
			want:        stagedUpgradeFinalize,
		},
		{
			name:        "one named worker: upgrade that worker",
			state:       stagedState(true, pending),
			upgradeNode: "node1",
			phase:       cluster.OperationPhaseAll,
			want:        stagedUpgradeResumeNode,
		},
		{
			// Only the Immutable kind has this rule. A load balancer is not a worker,
			// and the infrastructure phase upgrades it.
			name:        "one named load balancer: run the phases",
			state:       stagedState(true, pending),
			upgradeNode: "lb1",
			phase:       cluster.OperationPhaseAll,
			want:        stagedUpgradeProceed,
		},
		{
			name:        "a named host that the configuration does not hold: refuse",
			state:       stagedState(true, pending),
			upgradeNode: "ghost1",
			phase:       cluster.OperationPhaseAll,
			wantErr:     true,
		},
		{
			name:        "a selected phase leaves the rollout incomplete: refuse",
			state:       stagedState(true, pending),
			upgradeFlag: true,
			phase:       cluster.OperationPhaseDistribution,
			wantErr:     true,
		},
		{
			name:        "every worker succeeded: run the phases",
			state:       stagedState(true, map[string]upgrade.PhaseStatus{"node1": upgrade.PhaseStatusSuccess}),
			upgradeFlag: true,
			phase:       cluster.OperationPhaseAll,
			want:        stagedUpgradeResumeBatch,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			c := &ClusterCreator{
				furyctlConf:      labConf(),
				upgrade:          tc.upgradeFlag,
				upgradeNode:      tc.upgradeNode,
				skipNodesUpgrade: tc.skipNodesUpgrade,
				phase:            tc.phase,
			}

			got, err := c.stagedUpgradeDecision(tc.state, tc.changes, StartFromFlagNotSet)

			if tc.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// validateLoadedStagedUpgradeState refuses a state that furyctl cannot act on. The state
// lives in a ConfigMap of the cluster, so a hand edited one reaches this function.
func TestValidateLoadedStagedUpgradeState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		state   *upgrade.State
		wantErr bool
	}{
		{
			name:  "no staged worker",
			state: &upgrade.State{},
		},
		{
			name:  "a complete state",
			state: stagedState(true, map[string]upgrade.PhaseStatus{"node1": upgrade.PhaseStatusPending}),
		},
		{
			name: "staged workers without a transition",
			state: &upgrade.State{
				Phases: succeededPhases(),
				StagedWorkers: &upgrade.StagedWorkers{
					Nodes: map[string]upgrade.PhaseStatus{"node1": upgrade.PhaseStatusPending},
				},
			},
			wantErr: true,
		},
		{
			name:    "an unknown worker status",
			state:   stagedState(true, map[string]upgrade.PhaseStatus{"node1": "banana"}),
			wantErr: true,
		},
		{
			name: "ready to resume while a phase is incomplete",
			state: &upgrade.State{
				Phases: upgrade.Phases{
					PreKubernetes: &upgrade.Phase{Status: upgrade.PhaseStatusFailed},
				},
				Transition: &upgrade.Transition{From: "v1.35.1", To: "v1.36.0"},
				StagedWorkers: &upgrade.StagedWorkers{
					Nodes:          map[string]upgrade.PhaseStatus{"node1": upgrade.PhaseStatusPending},
					ReadyForResume: true,
				},
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := validateLoadedStagedUpgradeState(tc.state)

			if tc.wantErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, errStagedUpgrade)

				return
			}

			require.NoError(t, err)
		})
	}
}

// pendingStagedWorkersInConfigOrder upgrades the workers in the order of the configuration,
// and not in the order of a map, which Go does not keep.
func TestPendingStagedWorkersInConfigOrder(t *testing.T) {
	t.Parallel()

	c := &ClusterCreator{furyctlConf: labConf()}

	nodes := c.pendingStagedWorkersInConfigOrder(stagedState(true, map[string]upgrade.PhaseStatus{
		"node2":  upgrade.PhaseStatusPending,
		"node1":  upgrade.PhaseStatusPending,
		"ghost1": upgrade.PhaseStatusPending,
	}))

	// node1 and node2 in configuration order, and the unknown host last.
	assert.Equal(t, []string{"node1", "node2", "ghost1"}, nodes)
}
