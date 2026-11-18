// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package kfddistribution //nolint:testpackage // exercises the unexported distribution phase wiring.

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/apis/config"
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

// TestPruneBeforePlugins checks that --prune-plugins makes the plugins phase start with the prune: the
// distribution here has no prune.sh.tpl, so the prune fails before the plugins render, and the creator returns
// that error before StoreConfig.
func TestPruneBeforePlugins(t *testing.T) {
	t.Parallel()

	for _, prune := range []bool{true, false} {
		t.Run(fmt.Sprintf("prune %t", prune), func(t *testing.T) {
			t.Parallel()

			paths := cluster.CreatorPaths{
				ConfigPath: filepath.Join(t.TempDir(), "furyctl.yaml"),
				WorkDir:    t.TempDir(),
				DistroPath: t.TempDir(),
				BinPath:    t.TempDir(),
			}
			c := &ClusterCreator{paths: paths, prunePlugins: prune}
			plugins := commcreate.NewPlugins(paths, config.KFD{}, "Kind", false)

			c.pruneBeforePlugins(plugins, []commcreate.RemovedPlugin{{Kind: commcreate.PluginKindKustomize, Name: "my-plugin"}})

			err := plugins.Exec()

			require.Error(t, err)
			assert.Equal(t, prune, errors.Is(err, commcreate.ErrPruneScriptMissing))
		})
	}
}
