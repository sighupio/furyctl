// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package upgrade

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStagedWorkersProgress(t *testing.T) {
	t.Parallel()

	state := &State{
		StagedWorkers: &StagedWorkers{Nodes: map[string]PhaseStatus{
			"worker-c": PhaseStatusFailed,
			"worker-a": PhaseStatusSuccess,
			"worker-b": PhaseStatusPending,
		}},
	}

	assert.True(t, state.HasStagedWorkers())
	assert.Equal(t, []string{"worker-b", "worker-c"}, state.PendingStagedWorkers())
	assert.False(t, state.AllStagedWorkersSucceeded())

	state.MarkStagedWorker("worker-b", PhaseStatusSuccess)
	state.MarkStagedWorker("worker-c", PhaseStatusSuccess)

	assert.Empty(t, state.PendingStagedWorkers())
	assert.True(t, state.AllStagedWorkersSucceeded())
}

func TestAllStagedWorkersSucceededRejectsUnknownStatus(t *testing.T) {
	t.Parallel()

	state := &State{StagedWorkers: &StagedWorkers{Nodes: map[string]PhaseStatus{
		"worker-a": "unknown",
	}}}

	assert.False(t, state.AllStagedWorkersSucceeded())
}

func TestAllOnPremisesPhasesSucceeded(t *testing.T) {
	t.Parallel()

	succeeded := &Phase{Status: PhaseStatusSuccess}
	state := &State{Phases: Phases{
		PreKubernetes:    succeeded,
		Kubernetes:       succeeded,
		PostKubernetes:   succeeded,
		PreDistribution:  succeeded,
		Distribution:     succeeded,
		PostDistribution: succeeded,
	}}

	assert.True(t, state.AllOnPremisesPhasesSucceeded())

	state.Phases.PostDistribution = &Phase{Status: PhaseStatusFailed}
	assert.False(t, state.AllOnPremisesPhasesSucceeded())
}
