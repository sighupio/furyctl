// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package immutable //nolint:testpackage // exercises the unexported upgrade state reader.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/upgrade"
)

// readUpgradeState reads a stored upgrade state twice, and each reading answers a
// different question. This test pins both answers, because one reading for both is what
// makes a resumed upgrade start again from the first phase.
func TestReadUpgradeState(t *testing.T) {
	t.Parallel()

	creator := &ClusterCreator{upgradeStateStore: upgrade.NewStateStore("", "", "")}

	t.Run("a state that holds few phases gives every phase to the run", func(t *testing.T) {
		t.Parallel()

		// The state that `apply --upgrade --phase infrastructure` stores.
		raw := []byte(`phases:
  infrastructure:
    status: success
`)

		upgradeState, _, err := creator.readUpgradeState(raw, StartFromFlagNotSet)
		require.NoError(t, err)

		// The kubernetes phase writes PreKubernetes, which this state does not hold.
		// Without a complete state that write stops furyctl with a panic.
		phases := map[string]*upgrade.Phase{
			"Infrastructure":   upgradeState.Phases.Infrastructure,
			"PreKubernetes":    upgradeState.Phases.PreKubernetes,
			"Kubernetes":       upgradeState.Phases.Kubernetes,
			"PostKubernetes":   upgradeState.Phases.PostKubernetes,
			"PreDistribution":  upgradeState.Phases.PreDistribution,
			"Distribution":     upgradeState.Phases.Distribution,
			"PostDistribution": upgradeState.Phases.PostDistribution,
		}
		for name, phase := range phases {
			require.NotNilf(t, phase, "phase %s is absent, a write to its status panics", name)
		}

		// A stored status wins over the pending one.
		assert.Equal(t, upgrade.PhaseStatusSuccess, upgradeState.Phases.Infrastructure.Status)
		assert.Equal(t, upgrade.PhaseStatusPending, upgradeState.Phases.PreKubernetes.Status)
	})

	t.Run("the resume continues from the phase that stopped", func(t *testing.T) {
		t.Parallel()

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

		_, startFrom, err := creator.readUpgradeState(raw, StartFromFlagNotSet)
		require.NoError(t, err)
		assert.Equal(t, cluster.OperationPhaseDistribution, startFrom)
	})

	t.Run("a phase that the caller gives is kept", func(t *testing.T) {
		t.Parallel()

		_, startFrom, err := creator.readUpgradeState([]byte("phases: {}\n"), cluster.OperationPhaseKubernetes)
		require.NoError(t, err)
		assert.Equal(t, cluster.OperationPhaseKubernetes, startFrom)
	})
}
