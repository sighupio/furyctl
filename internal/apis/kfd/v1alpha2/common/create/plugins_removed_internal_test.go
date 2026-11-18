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

	"github.com/sighupio/furyctl/internal/cluster"
)

const removedPluginsErrorText = `These plugins were removed from spec.plugins, but their resources are still in the cluster:
  - my-plugin (kustomize)
  - cert-exporter (helm release, namespace monitoring)

furyctl stopped before changing anything. To continue, choose one:
  - Delete their resources:                furyctl apply --prune-plugins
  - Keep them managed by furyctl:          add the entries back to spec.plugins
  - Keep the resources outside of furyctl: furyctl apply --force plugins-unmanage`

const storedWithBothPlugins = `
spec:
  plugins:
    kustomize:
      - name: my-plugin
        folder: ./plugins/my-plugin
    helm:
      releases:
        - name: cert-exporter
          namespace: monitoring
          chart: enix/x509-certificate-exporter
`

// fakeConfigStore serves a stored configuration, or an error when none is stored.
type fakeConfigStore struct {
	cfg []byte
	err error
}

func (*fakeConfigStore) StoreKFD() error                    { return nil }
func (*fakeConfigStore) StoreConfig(map[string]any) error   { return nil }
func (f *fakeConfigStore) GetConfig() ([]byte, error)       { return f.cfg, f.err }
func (*fakeConfigStore) GetRenderedConfig() ([]byte, error) { return nil, errFake }

func kustomize(name string) RemovedPlugin {
	return RemovedPlugin{Kind: PluginKindKustomize, Name: name}
}

func helmRelease(namespace, name string) RemovedPlugin {
	return RemovedPlugin{Kind: PluginKindHelm, Name: name, Namespace: namespace}
}

func TestRemovedPlugins(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc    string
		stored  string
		current string
		want    []RemovedPlugin
	}{
		{
			desc: "a kustomize plugin is removed",
			stored: `
spec:
  plugins:
    kustomize: [{name: a, folder: ./a}, {name: b, folder: ./b}]
`,
			current: `
spec:
  plugins:
    kustomize: [{name: b, folder: ./b}]
`,
			want: []RemovedPlugin{kustomize("a")},
		},
		{
			desc: "reordering with other fields changed is not a removal",
			stored: `
spec:
  plugins:
    kustomize: [{name: a, folder: ./a}, {name: b, folder: ./b}]
    helm:
      releases:
        - {name: x, namespace: n1, chart: c1, values: [v1.yaml]}
        - {name: y, namespace: n2, chart: c2}
`,
			current: `
spec:
  plugins:
    kustomize: [{name: b, folder: ./other-b}, {name: a, folder: ./other-a}]
    helm:
      releases:
        - {name: y, namespace: n2, chart: other-c2}
        - {name: x, namespace: n1, chart: other-c1, values: [v2.yaml]}
`,
			want: []RemovedPlugin{},
		},
		{
			desc: "a helm release moves to another namespace",
			stored: `
spec:
  plugins:
    helm:
      releases: [{name: x, namespace: n1, chart: c}]
`,
			current: `
spec:
  plugins:
    helm:
      releases: [{name: x, namespace: n2, chart: c}]
`,
			want: []RemovedPlugin{helmRelease("n1", "x")},
		},
		{
			desc: "the whole spec.plugins is removed, kustomize first, each group in stored order",
			stored: `
spec:
  plugins:
    helm:
      releases: [{name: x, namespace: monitoring, chart: c}, {name: w, namespace: monitoring, chart: c}]
    kustomize: [{name: b, folder: ./b}, {name: a, folder: ./a}]
`,
			current: `
spec:
  distribution: {}
`,
			want: []RemovedPlugin{kustomize("b"), kustomize("a"), helmRelease("monitoring", "x"), helmRelease("monitoring", "w")},
		},
		{
			desc:    "a stored configuration without plugins removes nothing",
			stored:  ``,
			current: `spec: {plugins: {kustomize: [{name: a}]}}`,
			want:    []RemovedPlugin{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()

			got, err := RemovedPlugins([]byte(tc.stored), []byte(tc.current))

			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestRemovedPluginsInvalidConfig(t *testing.T) {
	t.Parallel()

	_, err := RemovedPlugins([]byte("spec: ["), []byte(""))

	require.ErrorContains(t, err, "stored configuration")
}

func TestRemovedPluginsErrorText(t *testing.T) {
	t.Parallel()

	err := &RemovedPluginsError{Plugins: []RemovedPlugin{
		kustomize("my-plugin"),
		helmRelease("monitoring", "cert-exporter"),
	}}

	assert.Equal(t, removedPluginsErrorText, err.Error())
}

func TestCheckRemovedPlugins(t *testing.T) {
	t.Parallel()

	removed := []RemovedPlugin{kustomize("my-plugin"), helmRelease("monitoring", "cert-exporter")}

	testCases := []struct {
		desc      string
		store     *fakeConfigStore
		current   map[string]any
		force     []string
		prune     bool
		want      []RemovedPlugin
		wantBlock bool
	}{
		{
			desc:  "no stored configuration skips the check",
			store: &fakeConfigStore{err: errFake},
			want:  nil,
		},
		{
			desc:      "a removal without a flag blocks",
			store:     &fakeConfigStore{cfg: []byte(storedWithBothPlugins)},
			want:      removed,
			wantBlock: true,
		},
		{
			desc:      "force migrations does not decide removed plugins",
			store:     &fakeConfigStore{cfg: []byte(storedWithBothPlugins)},
			force:     []string{cluster.ForceFeatureMigrations},
			want:      removed,
			wantBlock: true,
		},
		{
			desc:  "force plugins-unmanage continues",
			store: &fakeConfigStore{cfg: []byte(storedWithBothPlugins)},
			force: []string{cluster.ForceFeaturePluginsUnmanage},
			want:  removed,
		},
		{
			desc:  "force all continues",
			store: &fakeConfigStore{cfg: []byte(storedWithBothPlugins)},
			force: []string{cluster.ForceFeatureAll},
			want:  removed,
		},
		{
			desc:  "prune plugins continues",
			store: &fakeConfigStore{cfg: []byte(storedWithBothPlugins)},
			prune: true,
			want:  removed,
		},
		{
			desc:  "restored entries do not block",
			store: &fakeConfigStore{cfg: []byte(storedWithBothPlugins)},
			current: map[string]any{"spec": map[string]any{"plugins": map[string]any{
				"kustomize": []any{map[string]any{"name": "my-plugin"}},
				"helm": map[string]any{"releases": []any{
					map[string]any{"name": "cert-exporter", "namespace": "monitoring"},
				}},
			}}},
			want: []RemovedPlugin{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()

			got, err := CheckRemovedPlugins(tc.store, tc.current, tc.force, tc.prune)

			assert.Equal(t, tc.want, got)

			if !tc.wantBlock {
				require.NoError(t, err)

				return
			}

			var removedErr *RemovedPluginsError

			require.ErrorAs(t, err, &removedErr)
			assert.Equal(t, removedPluginsErrorText, err.Error())
		})
	}
}

func TestPreFlightError(t *testing.T) {
	t.Parallel()

	removedErr := &RemovedPluginsError{Plugins: []RemovedPlugin{kustomize("my-plugin")}}

	assert.Equal(t, removedErr.Error(), PreFlightError(removedErr).Error())

	err := PreFlightError(errFake)

	require.ErrorIs(t, err, errFake)
	assert.Equal(t, "error while executing preflight phase: fake error", err.Error())
	assert.False(t, errors.As(err, new(*RemovedPluginsError)))
}
