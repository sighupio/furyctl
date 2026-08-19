// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package cluster_test

import (
	"testing"

	r3diff "github.com/r3labs/diff/v3"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/cluster"
)

func TestAssertPhaseDiffs(t *testing.T) {
	t.Parallel()

	allPhases := []string{
		cluster.OperationPhaseInfrastructure,
		cluster.OperationPhaseKubernetes,
		cluster.OperationPhaseDistribution,
		cluster.OperationPhasePlugins,
	}

	changelog := func(phase string) r3diff.Changelog {
		return r3diff.Changelog{{Path: []string{"spec", phase, "someField"}}}
	}

	testCases := []struct {
		desc      string
		changed   string
		phase     string
		startFrom string
		phases    []string
		wantErr   error
	}{
		{
			desc:    "no phase and no start-from accepts any change",
			changed: "infrastructure",
			phases:  allPhases,
		},
		{
			desc:    "phase rejects changes to another phase",
			changed: "kubernetes",
			phase:   cluster.OperationPhaseDistribution,
			phases:  allPhases,
			wantErr: cluster.ErrChangesToOtherPhases,
		},
		{
			desc:    "phase accepts changes to the selected phase",
			changed: "distribution",
			phase:   cluster.OperationPhaseDistribution,
			phases:  allPhases,
		},
		{
			desc:      "start-from rejects changes to an earlier phase",
			changed:   "kubernetes",
			startFrom: cluster.OperationPhaseDistribution,
			phases:    allPhases,
			wantErr:   cluster.ErrChangesToSkippedPhases,
		},
		{
			desc:      "start-from accepts changes to the phase it starts from",
			changed:   "distribution",
			startFrom: cluster.OperationPhaseDistribution,
			phases:    allPhases,
		},
		{
			desc:      "start-from accepts changes to a later phase",
			changed:   "plugins",
			startFrom: cluster.OperationPhaseDistribution,
			phases:    allPhases,
		},
		{
			desc:      "start-from on a pre sub-phase accepts changes to its own phase",
			changed:   "kubernetes",
			startFrom: cluster.OperationSubPhasePreKubernetes,
			phases:    allPhases,
			wantErr:   nil,
		},
		{
			desc:      "start-from on a post sub-phase rejects changes to its own phase",
			changed:   "kubernetes",
			startFrom: cluster.OperationSubPhasePostKubernetes,
			phases:    allPhases,
			wantErr:   cluster.ErrChangesToSkippedPhases,
		},
		{
			desc:      "start-from of a phase the kind does not support is ignored",
			changed:   "kubernetes",
			startFrom: cluster.OperationPhaseInfrastructure,
			phases:    []string{cluster.OperationPhaseDistribution, cluster.OperationPhasePlugins},
		},
	}

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			err := cluster.AssertPhaseDiffs(changelog(tC.changed), tC.phase, tC.startFrom, tC.phases)

			if tC.wantErr == nil {
				require.NoError(t, err)

				return
			}

			require.ErrorIs(t, err, tC.wantErr)
		})
	}
}
