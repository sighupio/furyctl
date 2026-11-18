// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package create

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	commcreate "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/common/create"
	"github.com/sighupio/furyctl/internal/cluster"
)

var errNoStoredConfig = errors.New("secret furyctl-config not found")

// fakeConfigStore serves a stored configuration, or an error when none is stored.
type fakeConfigStore struct {
	cfg []byte
	err error
}

func (*fakeConfigStore) StoreKFD() error                    { return nil }
func (*fakeConfigStore) StoreConfig(map[string]any) error   { return nil }
func (f *fakeConfigStore) GetConfig() ([]byte, error)       { return f.cfg, f.err }
func (*fakeConfigStore) GetRenderedConfig() ([]byte, error) { return nil, errNoStoredConfig }

func TestPreFlightRemovedPlugins(t *testing.T) {
	t.Parallel()

	stored := []byte("spec: {plugins: {kustomize: [{name: my-plugin}, {name: b}]}}")
	rendered := map[string]any{"spec": map[string]any{"plugins": map[string]any{
		"kustomize": []any{map[string]any{"name": "b"}},
	}}}
	removed := []commcreate.RemovedPlugin{{Kind: commcreate.PluginKindKustomize, Name: "my-plugin"}}

	testCases := []struct {
		desc      string
		store     *fakeConfigStore
		force     []string
		prune     bool
		want      []commcreate.RemovedPlugin
		wantBlock bool
	}{
		{
			desc:      "a removal without a flag blocks",
			store:     &fakeConfigStore{cfg: stored},
			want:      removed,
			wantBlock: true,
		},
		{
			desc:  "force plugins-unmanage continues",
			store: &fakeConfigStore{cfg: stored},
			force: []string{cluster.ForceFeaturePluginsUnmanage},
			want:  removed,
		},
		{
			desc:  "prune plugins continues",
			store: &fakeConfigStore{cfg: stored},
			prune: true,
			want:  removed,
		},
		{
			desc:  "no stored configuration skips the check",
			store: &fakeConfigStore{err: errNoStoredConfig},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()

			p := &PreFlight{stateStore: tc.store, force: tc.force, prunePlugins: tc.prune}
			status := &Status{}

			err := p.checkRemovedPlugins(rendered, status)

			assert.Equal(t, tc.want, status.RemovedPlugins)
			assert.Equal(t, !tc.wantBlock, status.Success)

			if !tc.wantBlock {
				require.NoError(t, err)

				return
			}

			var removedErr *commcreate.RemovedPluginsError

			require.ErrorAs(t, err, &removedErr)
			assert.Equal(t, removed, removedErr.Plugins)
		})
	}
}
