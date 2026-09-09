// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package onpremises

import (
	"errors"
	"fmt"
	"testing"

	r3diff "github.com/r3labs/diff/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/onpremises/public"
	"github.com/sighupio/furyctl/internal/upgrade"
	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

func TestClusterCreatorWorkerNodes(t *testing.T) {
	t.Parallel()

	creator := &ClusterCreator{furyctlConf: public.OnpremisesKfdV1Alpha2{
		Spec: public.Spec{Kubernetes: public.Kubernetes{Nodes: []public.NodeGroup{
			{Name: "workers-a", Hosts: []public.Host{{Name: "worker-b"}, {Name: "worker-a"}}},
			{Name: "workers-b", Hosts: []public.Host{{Name: "worker-c"}}},
		}}},
	}}

	nodes := creator.workerNodes()
	assert.Equal(t, []string{"worker-b", "worker-a", "worker-c"}, nodes)
}

func TestPendingStagedWorkersFollowConfigOrder(t *testing.T) {
	t.Parallel()

	creator := &ClusterCreator{furyctlConf: public.OnpremisesKfdV1Alpha2{
		Spec: public.Spec{Kubernetes: public.Kubernetes{Nodes: []public.NodeGroup{
			{Name: "storage", Hosts: []public.Host{{Name: "zeta"}, {Name: "alfa"}}},
			{Name: "compute", Hosts: []public.Host{{Name: "beta"}}},
		}}},
	}}

	state := completedStagedState(map[string]upgrade.PhaseStatus{
		"alfa": upgrade.PhaseStatusPending,
		"beta": upgrade.PhaseStatusFailed,
		"zeta": upgrade.PhaseStatusPending,
	})

	assert.Equal(t, []string{"zeta", "alfa", "beta"}, creator.pendingStagedWorkersInConfigOrder(state))

	state.MarkStagedWorker("zeta", upgrade.PhaseStatusSuccess)
	assert.Equal(t, []string{"alfa", "beta"}, creator.pendingStagedWorkersInConfigOrder(state))
}

func TestPendingStagedWorkersKeepNodesMissingFromConfig(t *testing.T) {
	t.Parallel()

	creator := &ClusterCreator{furyctlConf: public.OnpremisesKfdV1Alpha2{
		Spec: public.Spec{Kubernetes: public.Kubernetes{Nodes: []public.NodeGroup{
			{Name: "compute", Hosts: []public.Host{{Name: "beta"}}},
		}}},
	}}

	state := completedStagedState(map[string]upgrade.PhaseStatus{
		"beta":    upgrade.PhaseStatusPending,
		"orphan":  upgrade.PhaseStatusPending,
		"orphan2": upgrade.PhaseStatusFailed,
	})

	assert.Equal(t, []string{"beta", "orphan", "orphan2"}, creator.pendingStagedWorkersInConfigOrder(state))
}

func TestStagedUpgradeDecision(t *testing.T) {
	t.Parallel()

	matchingVersion := r3diff.Changelog{{
		Path: []string{"spec", "distributionVersion"},
		From: "v1.33.1",
		To:   "v1.34.1",
	}}
	versionPlusDefaults := r3diff.Changelog{
		{Path: []string{"spec", "distribution", "customResources"}, From: nil, To: []any{}},
		{Path: []string{"spec", "distributionVersion"}, From: "v1.33.1", To: "v1.34.1"},
	}
	onlyDefaults := r3diff.Changelog{
		{Path: []string{"spec", "distribution", "customResources"}, From: nil, To: []any{}},
	}
	incomplete := completedStagedState(map[string]upgrade.PhaseStatus{"worker-a": upgrade.PhaseStatusPending})
	ready := completedStagedState(map[string]upgrade.PhaseStatus{"worker-a": upgrade.PhaseStatusPending})
	ready.StagedWorkers.ReadyForResume = true

	tests := []struct {
		name        string
		creator     ClusterCreator
		state       *upgrade.State
		changes     r3diff.Changelog
		wantAction  stagedUpgradeAction
		wantErr     bool
		errContains string
	}{
		{"no staged state", ClusterCreator{upgrade: true}, nil, nil, stagedUpgradeProceed, false, ""},
		{"plain apply", ClusterCreator{}, incomplete, nil, stagedUpgradeProceed, true, "worker upgrade is pending"},
		{"finalize skipped workers", ClusterCreator{upgrade: true, skipNodesUpgrade: true}, incomplete, versionPlusDefaults, stagedUpgradeFinalize, false, ""},
		{"recover finalization without diff", ClusterCreator{upgrade: true}, incomplete, nil, stagedUpgradeFinalize, false, ""},
		{"finalize matching diff", ClusterCreator{upgrade: true}, incomplete, matchingVersion, stagedUpgradeFinalize, false, ""},
		{"finalize matching diff plus defaults", ClusterCreator{upgrade: true}, incomplete, versionPlusDefaults, stagedUpgradeFinalize, false, ""},
		{"incomplete changes without version bump", ClusterCreator{upgrade: true}, incomplete, onlyDefaults, stagedUpgradeProceed, true, "does not request the recorded transition"},
		{"ready batch resume", ClusterCreator{upgrade: true}, ready, nil, stagedUpgradeResumeBatch, false, ""},
		{"ready skip workers", ClusterCreator{upgrade: true, skipNodesUpgrade: true}, ready, nil, stagedUpgradeNoop, false, ""},
		{"ready changed config", ClusterCreator{upgrade: true}, ready, matchingVersion, stagedUpgradeProceed, true, "configuration changed"},
		{"ready selected worker", ClusterCreator{upgradeNode: "worker-a"}, ready, nil, stagedUpgradeResumeNode, false, ""},
		{"ready selected worker with skip", ClusterCreator{upgradeNode: "worker-a", skipNodesUpgrade: true}, ready, nil, stagedUpgradeResumeNode, false, ""},
		{"selected worker changed config", ClusterCreator{upgradeNode: "worker-a"}, ready, matchingVersion, stagedUpgradeProceed, true, "configuration changed"},
		{"ready state with phase", ClusterCreator{upgrade: true, phase: "distribution"}, ready, nil, stagedUpgradeProceed, true, "without --phase"},
		{"ready state with skip and phase", ClusterCreator{upgrade: true, skipNodesUpgrade: true, phase: "distribution"}, ready, nil, stagedUpgradeProceed, true, "without --phase"},
		{"ready state with post apply phases", ClusterCreator{upgrade: true, postApplyPhases: []string{"distribution"}}, ready, nil, stagedUpgradeProceed, true, "--post-apply-phases"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			action, err := test.creator.stagedUpgradeDecision(test.state, test.changes, "")
			assert.Equal(t, test.wantAction, action)
			assert.Equal(t, test.wantErr, err != nil)
			if test.errContains != "" {
				assert.ErrorContains(t, err, test.errContains)
			}
		})
	}

	action, err := (&ClusterCreator{upgrade: true}).stagedUpgradeDecision(ready, nil, "distribution")
	assert.ErrorContains(t, err, "without --phase")
	assert.Equal(t, stagedUpgradeProceed, action)
}

func TestResumeStagedWorkers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		failNode     string
		wantErr      bool
		wantStates   []map[string]upgrade.PhaseStatus
		wantFinalize bool
		wantStores   int
	}{
		{
			name: "success",
			wantStates: []map[string]upgrade.PhaseStatus{
				{"worker-a": upgrade.PhaseStatusSuccess, "worker-b": upgrade.PhaseStatusPending},
				{"worker-a": upgrade.PhaseStatusSuccess, "worker-b": upgrade.PhaseStatusSuccess},
			},
			wantFinalize: true,
			wantStores:   1,
		},
		{
			name:     "failure",
			failNode: "worker-b",
			wantErr:  true,
			wantStates: []map[string]upgrade.PhaseStatus{
				{"worker-a": upgrade.PhaseStatusSuccess, "worker-b": upgrade.PhaseStatusPending},
				{"worker-a": upgrade.PhaseStatusSuccess, "worker-b": upgrade.PhaseStatusFailed},
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			state := completedStagedState(map[string]upgrade.PhaseStatus{
				"worker-a": upgrade.PhaseStatusPending,
				"worker-b": upgrade.PhaseStatusPending,
			})
			upgradeStore := &fakeUpgradeStorer{}
			configStore := &fakeStateStorer{}
			creator := &ClusterCreator{upgradeStateStore: upgradeStore, stateStore: configStore}
			workerUpgrader := &fakeWorkerUpgrader{failNode: test.failNode}

			err := creator.resumeStagedWorkers(
				workerUpgrader,
				state,
				[]string{"worker-a", "worker-b"},
				map[string]any{"spec": "target"},
			)
			assert.Equal(t, test.wantErr, err != nil)
			assert.Equal(t, []string{"worker-a", "worker-b"}, workerUpgrader.nodes)
			assert.Equal(t, test.wantStates, upgradeStore.storedWorkerStates)
			assert.Equal(t, test.wantFinalize, upgradeStore.deleted)
			assert.Equal(t, test.wantStores, configStore.storeConfigCalls)
			assert.Equal(t, test.wantStores, configStore.storeKFDCalls)
		})
	}
}

func TestPersistStagedUpgradeReady(t *testing.T) {
	t.Parallel()

	upgradeState := completedStagedState(map[string]upgrade.PhaseStatus{
		"worker-a": upgrade.PhaseStatusPending,
	})
	upgradeStore := &fakeUpgradeStorer{}
	configStore := &fakeStateStorer{}
	creator := &ClusterCreator{upgradeStateStore: upgradeStore, stateStore: configStore}

	err := creator.persistStagedUpgradeReady(upgradeState, map[string]any{"spec": "target"})
	require.NoError(t, err)
	assert.True(t, upgradeState.StagedWorkers.ReadyForResume)
	assert.Equal(t, []bool{true}, upgradeStore.storedReadyStates)
	assert.Equal(t, 1, configStore.storeConfigCalls)
	assert.Equal(t, 1, configStore.storeKFDCalls)
}

func TestPersistStagedUpgradeReadyCanBeRetried(t *testing.T) {
	t.Parallel()

	state := completedStagedState(map[string]upgrade.PhaseStatus{
		"worker-a": upgrade.PhaseStatusPending,
	})
	upgradeStore := &fakeUpgradeStorer{storeErr: errors.New("simulated state write failure")}
	configStore := &fakeStateStorer{}
	creator := &ClusterCreator{upgrade: true, upgradeStateStore: upgradeStore, stateStore: configStore}

	err := creator.persistStagedUpgradeReady(state, map[string]any{"spec": "target"})
	require.Error(t, err)
	assert.Equal(t, 1, configStore.storeConfigCalls)
	assert.Equal(t, 1, configStore.storeKFDCalls)

	// The failed write leaves the remote representation not ready even though
	// the local object was mutated before Store returned.
	state.StagedWorkers.ReadyForResume = false
	upgradeStore.storeErr = nil

	action, err := creator.stagedUpgradeDecision(state, nil, "")
	require.NoError(t, err)
	assert.Equal(t, stagedUpgradeFinalize, action)

	require.NoError(t, creator.persistStagedUpgradeReady(state, map[string]any{"spec": "target"}))
	assert.True(t, state.StagedWorkers.ReadyForResume)
	assert.Equal(t, 2, configStore.storeConfigCalls)
	assert.Equal(t, 2, configStore.storeKFDCalls)
}

func TestPersistPhaseScopedStagedUpgradeReady(t *testing.T) {
	t.Parallel()

	succeeded := func() *upgrade.Phase {
		return &upgrade.Phase{Status: upgrade.PhaseStatusSuccess}
	}
	state := &upgrade.State{
		Transition: &upgrade.Transition{From: "v1.33.1", To: "v1.34.1"},
		StagedWorkers: &upgrade.StagedWorkers{Nodes: map[string]upgrade.PhaseStatus{
			"worker-a": upgrade.PhaseStatusPending,
		}},
		Phases: upgrade.Phases{
			PreKubernetes:  succeeded(),
			Kubernetes:     succeeded(),
			PostKubernetes: succeeded(),
		},
	}
	upgradeStore := &fakeUpgradeStorer{}
	configStore := &fakeStateStorer{}
	creator := &ClusterCreator{upgradeStateStore: upgradeStore, stateStore: configStore}

	require.NoError(t, creator.persistAppliedConfig(
		state,
		&upgrade.Upgrade{},
		map[string]any{"spec": "target"},
	))
	assert.True(t, state.StagedWorkers.ReadyForResume)
	assert.Equal(t, 1, configStore.storeConfigCalls)
	assert.Equal(t, 1, configStore.storeKFDCalls)
	assert.False(t, upgradeStore.deleted)
}

func TestPersistAppliedConfigRejectsIncompleteStagedUpgrade(t *testing.T) {
	t.Parallel()

	upgradeState := completedStagedState(map[string]upgrade.PhaseStatus{
		"worker-a": upgrade.PhaseStatusPending,
	})
	upgradeState.Phases.Distribution.Status = upgrade.PhaseStatusFailed
	upgradeStore := &fakeUpgradeStorer{}
	configStore := &fakeStateStorer{}
	creator := &ClusterCreator{upgradeStateStore: upgradeStore, stateStore: configStore}

	err := creator.persistAppliedConfig(upgradeState, &upgrade.Upgrade{}, map[string]any{"spec": "target"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, errStagedUpgrade))
	assert.Zero(t, configStore.storeConfigCalls)
	assert.Zero(t, configStore.storeKFDCalls)
	assert.False(t, upgradeStore.deleted)
}

func TestLoadUpgradeStateRejectsInvalidWorkerProgress(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*upgrade.State)
	}{
		{"missing transition", func(state *upgrade.State) { state.Transition = nil }},
		{"ready with incomplete phases", func(state *upgrade.State) {
			state.StagedWorkers.ReadyForResume = true
			state.Phases.Distribution.Status = upgrade.PhaseStatusFailed
		}},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			state := completedStagedState(map[string]upgrade.PhaseStatus{"worker-a": upgrade.PhaseStatusPending})
			test.mutate(state)
			rawState, err := yamlx.MarshalV3(state)
			require.NoError(t, err)

			creator := &ClusterCreator{upgradeStateStore: &fakeUpgradeStorer{rawState: rawState}}
			_, _, err = creator.loadUpgradeState()
			require.Error(t, err)
			assert.True(t, errors.Is(err, errStagedUpgrade))
		})
	}
}

func completedStagedState(nodes map[string]upgrade.PhaseStatus) *upgrade.State {
	succeeded := func() *upgrade.Phase {
		return &upgrade.Phase{Status: upgrade.PhaseStatusSuccess}
	}

	return &upgrade.State{
		Transition:    &upgrade.Transition{From: "v1.33.1", To: "v1.34.1"},
		StagedWorkers: &upgrade.StagedWorkers{Nodes: nodes},
		Phases: upgrade.Phases{
			PreKubernetes:    succeeded(),
			Kubernetes:       succeeded(),
			PostKubernetes:   succeeded(),
			PreDistribution:  succeeded(),
			Distribution:     succeeded(),
			PostDistribution: succeeded(),
		},
	}
}

type fakeWorkerUpgrader struct {
	nodes    []string
	failNode string
}

func (f *fakeWorkerUpgrader) UpgradeWorkerNodes(
	nodes []string,
	onResult func(node string, status upgrade.PhaseStatus) error,
) error {
	f.nodes = append(f.nodes, nodes...)

	for _, node := range nodes {
		status := upgrade.PhaseStatusSuccess
		if node == f.failNode {
			status = upgrade.PhaseStatusFailed
		}

		if err := onResult(node, status); err != nil {
			return err
		}

		if status == upgrade.PhaseStatusFailed {
			return fmt.Errorf("simulated worker failure")
		}
	}

	return nil
}

type fakeUpgradeStorer struct {
	storedWorkerStates []map[string]upgrade.PhaseStatus
	storedReadyStates  []bool
	rawState           []byte
	deleted            bool
	storeErr           error
}

func (f *fakeUpgradeStorer) Store(state *upgrade.State) error {
	if f.storeErr != nil {
		return f.storeErr
	}

	workers := make(map[string]upgrade.PhaseStatus, len(state.StagedWorkers.Nodes))
	for node, status := range state.StagedWorkers.Nodes {
		workers[node] = status
	}
	f.storedWorkerStates = append(f.storedWorkerStates, workers)
	f.storedReadyStates = append(f.storedReadyStates, state.StagedWorkers.ReadyForResume)

	return nil
}

func (f *fakeUpgradeStorer) Get() ([]byte, error) {
	if f.rawState != nil {
		return f.rawState, nil
	}

	return nil, upgrade.ErrStateNotFound
}

func (f *fakeUpgradeStorer) Delete() error {
	f.deleted = true

	return nil
}

func (*fakeUpgradeStorer) GetLatestResumablePhase(*upgrade.State) string { return "" }

type fakeStateStorer struct {
	storeConfigCalls int
	storeKFDCalls    int
}

func (f *fakeStateStorer) StoreKFD() error {
	f.storeKFDCalls++

	return nil
}

func (f *fakeStateStorer) StoreConfig(map[string]any) error {
	f.storeConfigCalls++

	return nil
}

func (*fakeStateStorer) GetConfig() ([]byte, error)         { return nil, nil }
func (*fakeStateStorer) GetRenderedConfig() ([]byte, error) { return nil, nil }
