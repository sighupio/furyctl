// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package create //nolint:testpackage // exercises the unexported staging step.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/upgrade"
)

// stageWorkers records the workers that a run with --skip-nodes-upgrade does not upgrade.
// Every condition must hold, because a state with no staged worker reports no error. The
// finalize step then stores the target configuration and the workers stay on the old one.
func TestStageWorkers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		skipNodesUpgrade bool
		workerNodes      []string
		includesWorkers  bool
		wantNodes        []string
	}{
		{
			name:             "every condition holds",
			skipNodesUpgrade: true,
			workerNodes:      []string{"worker01", "worker02"},
			includesWorkers:  true,
			wantNodes:        []string{"worker01", "worker02"},
		},
		{
			name:             "the user did not skip the nodes",
			skipNodesUpgrade: false,
			workerNodes:      []string{"worker01"},
			includesWorkers:  true,
		},
		{
			name:             "the configuration lists no worker",
			skipNodesUpgrade: true,
			workerNodes:      nil,
			includesWorkers:  true,
		},
		{
			name:             "the upgrade path upgrades no worker",
			skipNodesUpgrade: true,
			workerNodes:      []string{"worker01"},
			includesWorkers:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			k := &Kubernetes{
				skipNodesUpgrade: tc.skipNodesUpgrade,
				workerNodes:      tc.workerNodes,
				upgrade: &upgrade.Upgrade{
					Enabled:               true,
					From:                  "v1.35.1",
					To:                    "v1.36.0",
					IncludesWorkerUpgrade: tc.includesWorkers,
				},
			}

			upgradeState := &upgrade.State{}

			k.stageWorkers(upgradeState)

			if tc.wantNodes == nil {
				assert.False(t, upgradeState.HasStagedWorkers())
				assert.Nil(t, upgradeState.Transition)

				return
			}

			require.True(t, upgradeState.HasStagedWorkers())
			assert.Equal(t, tc.wantNodes, upgradeState.PendingStagedWorkers())

			// The transition travels with the staged workers. The resume path has no
			// configuration diff to read the versions from.
			require.NotNil(t, upgradeState.Transition)
			assert.Equal(t, "v1.35.1", upgradeState.Transition.From)
			assert.Equal(t, "v1.36.0", upgradeState.Transition.To)
		})
	}
}
