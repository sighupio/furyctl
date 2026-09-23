// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package create //nolint:testpackage // exercises the unexported state of the preflight check.

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Each case is a state that the preflight playbook can write. The control plane holds cp1 and
// cp2, and w1 and w2 are workers.
func TestClusterStateAssess(t *testing.T) {
	t.Parallel()

	controlPlane := []string{"cp1", "cp2"}

	tests := []struct {
		name        string
		state       clusterState
		warnContain string
		errContain  string
	}{
		{
			name:        "empty, a first apply",
			state:       clusterState{Unreachable: []string{"cp1", "cp2", "w1", "w2"}},
			warnContain: "could not read these hosts: cp1, cp2, w1, w2. It continues as if the cluster does not exist",
		},
		{
			name:  "provisioned, every host answers and holds no cluster",
			state: clusterState{Reachable: []string{"cp1", "cp2", "w1", "w2"}},
		},
		{
			name: "partially provisioned, an interrupted first boot",
			state: clusterState{
				Reachable:   []string{"cp1", "cp2", "w1"},
				Unreachable: []string{"w2"},
			},
			warnContain: "could not read these hosts: w2. The other hosts answer and hold no cluster",
		},
		{
			name: "the workers joined a cluster and no control plane host answers",
			state: clusterState{
				Reachable:        []string{"w1", "w2"},
				Unreachable:      []string{"cp1", "cp2"},
				KubeletConfHosts: []string{"w1", "w2"},
			},
			errContain: "These control plane hosts do not answer: cp1, cp2. Make sure that they answer, then apply again. To make a new control plane, reset the hosts that joined first",
		},
		{
			name: "the control plane answers and lost the cluster that the workers joined",
			state: clusterState{
				Reachable:        []string{"cp1", "cp2", "w1", "w2"},
				KubeletConfHosts: []string{"w1", "w2"},
			},
			errContain: "no control plane host holds admin.conf",
		},
		{
			name: "cluster found",
			state: clusterState{
				Reachable:        []string{"cp1", "cp2", "w1", "w2"},
				KubeletConfHosts: []string{"cp1", "cp2", "w1", "w2"},
				AdminConfHosts:   []string{"cp2"},
			},
		},
		{
			name: "cluster found with one control plane host down",
			state: clusterState{
				Reachable:        []string{"cp2", "w1", "w2"},
				Unreachable:      []string{"cp1"},
				KubeletConfHosts: []string{"cp2", "w1", "w2"},
				AdminConfHosts:   []string{"cp2"},
			},
			warnContain: "These hosts do not answer: cp1. furyctl read the cluster from cp2 and continues. If these hosts are new, or you provision them again, boot them while the assets server waits",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			warning, err := test.state.assess(controlPlane)

			if test.errContain != "" {
				require.Error(t, err)
				assert.True(t, errors.Is(err, errClusterUnreachable))
				assert.Contains(t, err.Error(), test.errContain)
				assert.Empty(t, warning)

				return
			}

			require.NoError(t, err)

			if test.warnContain == "" {
				assert.Empty(t, warning)
			} else {
				assert.Contains(t, warning, test.warnContain)
			}
		})
	}
}

// A distribution released before the state file writes no file, and the check then keeps the
// behavior that it had before.
func TestReadClusterState(t *testing.T) {
	t.Parallel()

	t.Run("no file", func(t *testing.T) {
		t.Parallel()

		_, found, err := readClusterState(t.TempDir())

		require.NoError(t, err)
		assert.False(t, found)
	})

	t.Run("the file that the playbook writes", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		data := `{"reachable": ["w1"], "unreachable": ["cp1"], "kubeletConfHosts": ["w1"], "adminConfHosts": []}`
		require.NoError(t, os.WriteFile(filepath.Join(dir, clusterStateFile), []byte(data), 0o600))

		state, found, err := readClusterState(dir)

		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, clusterState{
			Reachable:        []string{"w1"},
			Unreachable:      []string{"cp1"},
			KubeletConfHosts: []string{"w1"},
			AdminConfHosts:   []string{},
		}, state)
	})

	t.Run("a file that is not JSON", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, clusterStateFile), []byte("{"), 0o600))

		_, _, err := readClusterState(dir)

		assert.ErrorContains(t, err, "error parsing state.json")
	})
}

func TestClusterStateSummary(t *testing.T) {
	t.Parallel()

	state := clusterState{
		Reachable:        []string{"cp2", "w1"},
		Unreachable:      []string{"cp1"},
		KubeletConfHosts: []string{"cp2", "w1"},
		AdminConfHosts:   []string{"cp2"},
	}

	assert.Equal(t,
		"Preflight state: 2 of 3 hosts answer, 2 joined a cluster, admin.conf found on cp2",
		state.summary(),
	)
}
