// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package onpremises

import (
	"errors"
	"fmt"
	"os"
	"testing"

	r3diff "github.com/r3labs/diff/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	commcreate "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/common/create"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/onpremises/public"
	"github.com/sighupio/furyctl/internal/cluster"
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
	failedPhase := completedStagedState(map[string]upgrade.PhaseStatus{"worker-a": upgrade.PhaseStatusPending})
	failedPhase.Phases.Distribution.Status = upgrade.PhaseStatusFailed
	stoppedBeforeStaging := &upgrade.State{Phases: completedStagedState(nil).Phases}
	stoppedBeforeStaging.Phases.PreKubernetes.Status = upgrade.PhaseStatusFailed
	stoppedBeforeStaging.Phases.Kubernetes.Status = upgrade.PhaseStatusPending
	otherVersion := r3diff.Changelog{{
		Path: []string{"spec", "distributionVersion"},
		From: "v1.33.1",
		To:   "v1.35.0",
	}}

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
		{"ready changed config", ClusterCreator{upgrade: true}, ready, matchingVersion, stagedUpgradeProceed, true, "revert the change"},
		// Every command that continues the rollout refuses a change, so each message gives the same path out.
		{"ready changed config, plain apply", ClusterCreator{}, ready, onlyDefaults, stagedUpgradeProceed, true, "revert the change"},
		// Before the state is ready, the difference holds the version change, so the message names --upgrade.
		{"version change before the state is ready, plain apply", ClusterCreator{}, incomplete, matchingVersion, stagedUpgradeProceed, true, "a worker upgrade is pending"},
		{"ready changed config, skip workers", ClusterCreator{upgrade: true, skipNodesUpgrade: true}, ready, onlyDefaults, stagedUpgradeProceed, true, "revert the change"},
		{"ready changed config, selected worker", ClusterCreator{upgradeNode: "worker-a"}, ready, onlyDefaults, stagedUpgradeProceed, true, "revert the change"},
		{"ready selected worker", ClusterCreator{upgradeNode: "worker-a"}, ready, nil, stagedUpgradeResumeNode, false, ""},
		{"ready selected worker with skip", ClusterCreator{upgradeNode: "worker-a", skipNodesUpgrade: true}, ready, nil, stagedUpgradeResumeNode, false, ""},
		{"selected worker changed config", ClusterCreator{upgradeNode: "worker-a"}, ready, matchingVersion, stagedUpgradeProceed, true, "configuration changed"},
		{"ready state with phase", ClusterCreator{upgrade: true, phase: "distribution"}, ready, nil, stagedUpgradeProceed, true, "without --phase"},
		{"ready state with skip and phase", ClusterCreator{upgrade: true, skipNodesUpgrade: true, phase: "distribution"}, ready, nil, stagedUpgradeProceed, true, "without --phase"},
		{"ready state with post apply phases", ClusterCreator{upgrade: true, postApplyPhases: []string{"distribution"}}, ready, nil, stagedUpgradeProceed, true, "--post-apply-phases"},
		{"forced phase", ClusterCreator{upgrade: true, phase: "distribution", force: []string{"upgrades"}}, ready, nil, stagedUpgradeProceed, false, ""},
		{"post apply phases forced for all", ClusterCreator{upgrade: true, postApplyPhases: []string{"distribution"}, force: []string{"all"}}, ready, nil, stagedUpgradeProceed, false, ""},
		{"forced phase with changes", ClusterCreator{upgrade: true, phase: "distribution", force: []string{"upgrades"}}, ready, matchingVersion, stagedUpgradeProceed, true, "configuration changed"},
		// stagedUpgradeDecision refuses a selected phase also when the rollout is not ready.
		{"selected phase with a failed phase", ClusterCreator{upgrade: true, phase: "distribution"}, failedPhase, matchingVersion, stagedUpgradeProceed, true, "without --phase"},
		{"selected phase before finalize", ClusterCreator{upgrade: true, phase: "distribution"}, incomplete, matchingVersion, stagedUpgradeProceed, true, "without --phase"},
		{"post apply phases before finalize", ClusterCreator{upgrade: true, postApplyPhases: []string{"distribution"}}, incomplete, matchingVersion, stagedUpgradeProceed, true, "--post-apply-phases"},
		{"selected worker after an upgrade that stopped before staging", ClusterCreator{upgradeNode: "worker-a"}, stoppedBeforeStaging, matchingVersion, stagedUpgradeProceed, true, "did not complete"},
		{"selected worker after a stopped upgrade with no version change", ClusterCreator{upgradeNode: "worker-a"}, stoppedBeforeStaging, nil, stagedUpgradeProceed, false, ""},
		// A run for one host with no staged worker goes to the phases, so it must find no change.
		{"selected worker, no state, a change", ClusterCreator{upgradeNode: "worker-a", phase: cluster.OperationPhaseAll}, nil, onlyDefaults, stagedUpgradeProceed, true, "run 'furyctl apply' to"},
		{"selected worker, no staged worker, a change", ClusterCreator{upgradeNode: "worker-a", phase: cluster.OperationPhaseAll}, &upgrade.State{Phases: completedStagedState(nil).Phases}, onlyDefaults, stagedUpgradeProceed, true, "run 'furyctl apply' to"},
		{"selected worker after an upgrade that stopped after staging", ClusterCreator{upgradeNode: "worker-a"}, failedPhase, matchingVersion, stagedUpgradeProceed, true, "did not complete"},
		// The refusal comes before the test of the transition. The next run gives that error.
		{"selected worker after a stopped upgrade to another version", ClusterCreator{upgradeNode: "worker-a"}, failedPhase, otherVersion, stagedUpgradeProceed, true, "did not complete"},
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

	action, err = (&ClusterCreator{upgrade: true, force: []string{"upgrades"}}).
		stagedUpgradeDecision(ready, nil, "distribution")
	assert.NoError(t, err)
	assert.Equal(t, stagedUpgradeProceed, action)
}

// persistStagedUpgradeReadyAfterPhases stores only when the run has staged workers and every
// tracked phase succeeded, and never in a dry run.
func TestPersistStagedUpgradeReadyAfterPhases(t *testing.T) {
	t.Parallel()

	pending := map[string]upgrade.PhaseStatus{"worker-a": upgrade.PhaseStatusPending}

	pendingPhase := completedStagedState(pending)
	pendingPhase.Phases.Distribution.Status = upgrade.PhaseStatusPending

	tests := []struct {
		name      string
		state     *upgrade.State
		dryRun    bool
		wantStore bool
	}{
		{name: "staged workers and every tracked phase succeeded", state: completedStagedState(pending), wantStore: true},
		{name: "no staged worker", state: &upgrade.State{}},
		{name: "a tracked phase did not succeed", state: pendingPhase},
		{name: "dry run", state: completedStagedState(pending), dryRun: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			upgradeStore := &fakeUpgradeStorer{}
			configStore := &fakeStateStorer{}
			creator := &ClusterCreator{
				dryRun:            test.dryRun,
				upgradeStateStore: upgradeStore,
				stateStore:        configStore,
				renderedConfig:    map[string]any{"spec": "target"},
			}

			require.NoError(t, creator.persistStagedUpgradeReadyAfterPhases(test.state))

			if !test.wantStore {
				assert.Zero(t, configStore.storeConfigCalls)
				assert.Empty(t, upgradeStore.storedReadyStates)

				return
			}

			assert.Equal(t, 1, configStore.storeConfigCalls)
			assert.Equal(t, []bool{true}, upgradeStore.storedReadyStates)
		})
	}
}

// stagedWorkersAdvice tells the operator how to continue only when workers are staged.
func TestStagedWorkersAdvice(t *testing.T) {
	t.Parallel()

	errStep := errors.New("error while executing plugins phase")

	staged := stagedWorkersAdvice(errStep, completedStagedState(map[string]upgrade.PhaseStatus{
		"worker-a": upgrade.PhaseStatusPending,
	}))
	require.ErrorIs(t, staged, errStep)
	assert.Contains(t, staged.Error(), "Run 'furyctl apply --upgrade' to upgrade them")

	assert.Equal(t, errStep, stagedWorkersAdvice(errStep, &upgrade.State{}))
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

func TestResumeStagedWorkerBatchDryRunDoesNotAskForConfirmation(t *testing.T) {
	state := completedStagedState(map[string]upgrade.PhaseStatus{
		"worker-a": upgrade.PhaseStatusPending,
	})
	upgradeStore := &fakeUpgradeStorer{}
	workerUpgrader := &fakeWorkerUpgrader{}
	creator := &ClusterCreator{
		dryRun:            true,
		upgradeStateStore: upgradeStore,
	}

	stdin, err := os.Open(os.DevNull)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, stdin.Close()) })

	originalStdin := os.Stdin
	os.Stdin = stdin
	t.Cleanup(func() { os.Stdin = originalStdin })

	require.NoError(t, creator.resumeStagedWorkerBatch(
		workerUpgrader,
		state,
		map[string]any{"spec": "target"},
	))
	assert.Equal(t, []string{"worker-a"}, workerUpgrader.nodes)
	assert.Empty(t, upgradeStore.storedWorkerStates)
	assert.False(t, upgradeStore.deleted)
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

// Create() wraps the distribution phase in the upgrade decorator, which hides StorageSkipper.
// The creator must keep the undecorated phase, or the distribution phase never runs again.
func TestNewDistributionPhaseKeepsTheStorageSkipper(t *testing.T) {
	t.Parallel()

	c := &ClusterCreator{}
	phase := c.newDistributionPhase(upgrade.New(cluster.CreatorPaths{}, "OnPremises"))

	require.NotNil(t, c.distribution)
	assert.Same(t, phase.Self(), c.distribution.Self(), "c.distribution must be the phase that the decorator wraps")

	var skipper commcreate.StorageSkipper = c.distribution
	assert.Empty(t, skipper.SkippedStoragePackages())

	_, ok := any(phase).(commcreate.StorageSkipper)
	assert.False(t, ok, "the decorator hides StorageSkipper, use c.distribution")
}

// A run for one host applies only its playbook, and then stores the configuration as applied.
// A change in the configuration is then lost, so the run refuses it. A run of a phase that does
// not use --upgrade-node keeps its changes.
func TestValidateUpgradeNodeChanges(t *testing.T) {
	t.Parallel()

	changes := r3diff.Changelog{{
		Type: "create",
		Path: []string{"spec", "distribution", "customPatches"},
		To:   map[string]any{},
	}}

	versionChange := r3diff.Changelog{{
		Path: []string{"spec", "distributionVersion"},
		From: "v1.35.1",
		To:   "v1.36.0",
	}}

	tests := []struct {
		name    string
		creator ClusterCreator
		changes r3diff.Changelog
		wantErr string
	}{
		{"no host, a change", ClusterCreator{phase: cluster.OperationPhaseAll}, changes, ""},
		{"a host, no change", ClusterCreator{upgradeNode: "worker-a", phase: cluster.OperationPhaseAll}, nil, ""},
		{"a host and a change", ClusterCreator{upgradeNode: "worker-a", phase: cluster.OperationPhaseAll}, changes, "run 'furyctl apply' to"},
		{"a host and a change, kubernetes phase", ClusterCreator{upgradeNode: "worker-a", phase: cluster.OperationPhaseKubernetes}, changes, "run 'furyctl apply' to"},
		{"a host and a change, distribution phase", ClusterCreator{upgradeNode: "worker-a", phase: cluster.OperationPhaseDistribution}, changes, ""},
		// A plain apply refuses a version change, so the advice names the upgrade.
		{"a host and a version change", ClusterCreator{upgradeNode: "worker-a", phase: cluster.OperationPhaseAll}, versionChange, "--upgrade --skip-nodes-upgrade"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := test.creator.validateUpgradeNodeChanges(test.changes)
			if test.wantErr != "" {
				assert.ErrorIs(t, err, errUpgradeNodeChanges)
				assert.ErrorContains(t, err, test.wantErr)

				return
			}

			assert.NoError(t, err)
		})
	}
}
