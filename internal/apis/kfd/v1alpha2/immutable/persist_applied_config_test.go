// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package immutable //nolint:testpackage // exercises the unexported finalize step.

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/immutable/create"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/upgrade"
)

type fakeUpgradeStore struct {
	stored  *upgrade.State
	deleted bool
}

func (f *fakeUpgradeStore) Store(state *upgrade.State) error { f.stored = state; return nil }
func (f *fakeUpgradeStore) Get() ([]byte, error)             { return nil, errors.New("not stored") }
func (f *fakeUpgradeStore) Delete() error                    { f.deleted = true; return nil }
func (*fakeUpgradeStore) GetLatestResumablePhase(*upgrade.State) string {
	return ""
}

type fakeConfigStore struct {
	storedConfig bool
	storedKFD    bool
}

func (f *fakeConfigStore) StoreKFD() error                  { f.storedKFD = true; return nil }
func (f *fakeConfigStore) StoreConfig(map[string]any) error { f.storedConfig = true; return nil }
func (*fakeConfigStore) GetConfig() ([]byte, error)         { return nil, nil }
func (*fakeConfigStore) GetRenderedConfig() ([]byte, error) { return nil, nil }

func succeededPhases() upgrade.Phases {
	done := func() *upgrade.Phase { return &upgrade.Phase{Status: upgrade.PhaseStatusSuccess} }

	return upgrade.Phases{
		PreInfrastructure: done(), Infrastructure: done(), PostInfrastructure: done(),
		PreKubernetes: done(), Kubernetes: done(), PostKubernetes: done(),
		PreDistribution: done(), Distribution: done(), PostDistribution: done(),
	}
}

// persistAppliedConfig decides what happens to the upgrade state at the end of a run. The
// decision keeps the cluster configuration and the version of the workers in agreement.
func TestPersistAppliedConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		state          *upgrade.State
		upgradeEnabled bool
		wantConfig     bool
		wantDeleted    bool
		wantReady      bool
		wantErr        bool
	}{
		{
			name:           "no staged worker and no upgrade: store the configuration",
			state:          &upgrade.State{},
			upgradeEnabled: false,
			wantConfig:     true,
		},
		{
			name:           "an upgrade without staged workers: delete the state",
			state:          &upgrade.State{Phases: succeededPhases()},
			upgradeEnabled: true,
			wantConfig:     true,
			wantDeleted:    true,
		},
		{
			name: "every phase and every worker succeeded: delete the state",
			state: &upgrade.State{
				Phases:     succeededPhases(),
				Transition: &upgrade.Transition{From: "v1.35.1", To: "v1.36.0"},
				StagedWorkers: &upgrade.StagedWorkers{Nodes: map[string]upgrade.PhaseStatus{
					"worker01": upgrade.PhaseStatusSuccess,
				}},
			},
			upgradeEnabled: true,
			wantConfig:     true,
			wantDeleted:    true,
		},
		{
			name: "workers stay behind: keep the state and mark it ready",
			state: &upgrade.State{
				Phases:     succeededPhases(),
				Transition: &upgrade.Transition{From: "v1.35.1", To: "v1.36.0"},
				StagedWorkers: &upgrade.StagedWorkers{Nodes: map[string]upgrade.PhaseStatus{
					"worker01": upgrade.PhaseStatusPending,
					"worker02": upgrade.PhaseStatusPending,
				}},
			},
			upgradeEnabled: true,
			wantConfig:     true,
			wantDeleted:    false,
			wantReady:      true,
		},
		{
			name: "a phase failed: store nothing and report the error",
			state: &upgrade.State{
				Phases: upgrade.Phases{
					PreKubernetes: &upgrade.Phase{Status: upgrade.PhaseStatusSuccess},
					Kubernetes:    &upgrade.Phase{Status: upgrade.PhaseStatusFailed},
				},
				Transition: &upgrade.Transition{From: "v1.35.1", To: "v1.36.0"},
				StagedWorkers: &upgrade.StagedWorkers{Nodes: map[string]upgrade.PhaseStatus{
					"worker01": upgrade.PhaseStatusPending,
				}},
			},
			upgradeEnabled: true,
			wantErr:        true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			upgradeStore := &fakeUpgradeStore{}
			configStore := &fakeConfigStore{}

			c := &ClusterCreator{
				upgradeStateStore: upgradeStore,
				stateStore:        configStore,
				phase:             cluster.OperationPhaseAll,
			}

			err := c.persistAppliedConfig(
				tc.state,
				&upgrade.Upgrade{Enabled: tc.upgradeEnabled},
				map[string]any{},
				&create.Status{ClusterExists: true},
			)

			if tc.wantErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, errStagedUpgrade)
				// A failed rollout must not record the target version.
				assert.False(t, configStore.storedConfig, "stored the configuration after a failed phase")
				assert.False(t, upgradeStore.deleted, "deleted the state after a failed phase")

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantConfig, configStore.storedConfig, "stored configuration")
			assert.Equal(t, tc.wantDeleted, upgradeStore.deleted, "deleted state")

			if tc.wantReady {
				require.NotNil(t, upgradeStore.stored, "kept no state for the next run")
				assert.True(t, upgradeStore.stored.StagedWorkers.ReadyForResume)
			}
		})
	}
}

// The infrastructure phase of a cluster that does not exist yet stores no configuration.
func TestPersistAppliedConfigSkipsAnAbsentCluster(t *testing.T) {
	t.Parallel()

	configStore := &fakeConfigStore{}
	c := &ClusterCreator{
		upgradeStateStore: &fakeUpgradeStore{},
		stateStore:        configStore,
		phase:             cluster.OperationPhaseInfrastructure,
	}

	err := c.persistAppliedConfig(
		&upgrade.State{},
		&upgrade.Upgrade{},
		map[string]any{},
		&create.Status{ClusterExists: false},
	)

	require.NoError(t, err)
	assert.False(t, configStore.storedConfig)
	assert.False(t, configStore.storedKFD)
}

// persistStagedUpgradeReadyAfterPhases stores only when the run has staged workers and every
// tracked phase succeeded, and never in a dry run.
func TestPersistStagedUpgradeReadyAfterPhases(t *testing.T) {
	t.Parallel()

	pending := map[string]upgrade.PhaseStatus{"node1": upgrade.PhaseStatusPending}

	pendingPhase := stagedState(false, pending)
	pendingPhase.Phases.Distribution.Status = upgrade.PhaseStatusPending

	tests := []struct {
		name      string
		state     *upgrade.State
		dryRun    bool
		wantStore bool
	}{
		{name: "staged workers and every tracked phase succeeded", state: stagedState(false, pending), wantStore: true},
		{name: "no staged worker", state: &upgrade.State{Phases: succeededPhases()}},
		{name: "a tracked phase did not succeed", state: pendingPhase},
		{name: "dry run", state: stagedState(false, pending), dryRun: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			upgradeStore := &fakeUpgradeStore{}
			configStore := &fakeConfigStore{}
			c := &ClusterCreator{
				dryRun:            tc.dryRun,
				upgradeStateStore: upgradeStore,
				stateStore:        configStore,
				renderedConfig:    map[string]any{"spec": "target"},
			}

			require.NoError(t, c.persistStagedUpgradeReadyAfterPhases(tc.state))
			assert.Equal(t, tc.wantStore, configStore.storedConfig)
			assert.Equal(t, tc.wantStore, upgradeStore.stored != nil)

			if tc.wantStore {
				assert.True(t, upgradeStore.stored.StagedWorkers.ReadyForResume)
			}
		})
	}
}

// stagedWorkersAdvice tells the operator how to continue only when workers are staged.
func TestStagedWorkersAdvice(t *testing.T) {
	t.Parallel()

	errStep := errors.New("error while executing plugins phase")

	staged := stagedWorkersAdvice(errStep, stagedState(true, map[string]upgrade.PhaseStatus{"node1": upgrade.PhaseStatusPending}))
	require.ErrorIs(t, staged, errStep)
	assert.Contains(t, staged.Error(), "Run 'furyctl apply --upgrade' to upgrade them")

	assert.Equal(t, errStep, stagedWorkersAdvice(errStep, &upgrade.State{}))
}
