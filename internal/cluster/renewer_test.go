// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package cluster_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/cluster"
)

// A distribution that does not have the playbook must fail before it runs ansible, with a message
// that names the SD version.
func TestRunPlaybookMissing(t *testing.T) {
	t.Parallel()

	err := cluster.RunPlaybook(nil, t.TempDir(), "v1.35.1", "renew-kubeconfigs.yaml")

	require.Error(t, err)
	assert.True(t, errors.Is(err, cluster.ErrUnsupportedByDistribution))
	assert.Contains(t, err.Error(), "v1.35.1")
	assert.Contains(t, err.Error(), "renew-kubeconfigs.yaml")
}
