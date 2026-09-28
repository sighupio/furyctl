// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package onpremises //nolint:testpackage // exercises the unexported delete decision.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/cluster"
)

// distributionDeletion never runs the distribution phase without a cluster, because that phase would
// then act on the cluster of the current kubeconfig context.
func TestDistributionDeletion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		phase         string
		clusterExists bool
		want          bool
		wantErr       error
	}{
		{name: "all phases with a cluster", phase: cluster.OperationPhaseAll, clusterExists: true, want: true},
		{name: "all phases without a cluster", phase: cluster.OperationPhaseAll},
		{name: "distribution with a cluster", phase: cluster.OperationPhaseDistribution, clusterExists: true, want: true},
		{name: "distribution without a cluster", phase: cluster.OperationPhaseDistribution, wantErr: errClusterNotFound},
		{name: "kubernetes with a cluster", phase: cluster.OperationPhaseKubernetes, clusterExists: true},
		{name: "kubernetes without a cluster", phase: cluster.OperationPhaseKubernetes},
		{name: "an unknown phase", phase: "plugins", wantErr: ErrUnsupportedPhase},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := distributionDeletion(tc.phase, tc.clusterExists)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
