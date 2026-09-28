// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package kfddistribution //nolint:testpackage // exercises the unexported distribution phase wiring.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	commcreate "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/common/create"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/upgrade"
)

// Create() wraps the distribution phase in the upgrade decorator, which hides StorageSkipper.
// The creator must keep the undecorated phase, or the distribution phase never runs again.
func TestNewDistributionPhaseKeepsTheStorageSkipper(t *testing.T) {
	t.Parallel()

	c := &ClusterCreator{}
	phase := c.newDistributionPhase(upgrade.New(cluster.CreatorPaths{}, "KFDDistribution"))

	require.NotNil(t, c.distribution)
	assert.Same(t, phase.Self(), c.distribution.Self(), "c.distribution must be the phase that the decorator wraps")

	var skipper commcreate.StorageSkipper = c.distribution
	assert.Empty(t, skipper.SkippedStoragePackages())

	_, ok := any(phase).(commcreate.StorageSkipper)
	assert.False(t, ok, "the decorator hides StorageSkipper, use c.distribution")
}
