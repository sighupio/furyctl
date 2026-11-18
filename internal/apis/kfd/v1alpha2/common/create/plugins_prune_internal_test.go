// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package create

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/apis/config"
	"github.com/sighupio/furyctl/internal/cluster"
)

const storedWithRelease = `
apiVersion: kfd.sighup.io/v1alpha2
kind: KFDDistribution
spec:
  distribution: {}
  plugins:
    helm:
      releases:
        - name: x
          namespace: n
          chart: c
`

// fakeScriptRunner records the prune.sh calls and fails the ones whose mode is in fail.
type fakeScriptRunner struct {
	calls [][]string
	fail  map[string]error
}

func (f *fakeScriptRunner) Run(args ...string) (string, error) {
	f.calls = append(f.calls, args)

	if err := f.fail[args[1]]; err != nil {
		return "", err
	}

	return "Plugin x (helm release, namespace n): installed", nil
}

func (f *fakeScriptRunner) modes() []string {
	m := []string{}
	for _, c := range f.calls {
		m = append(m, strings.Join(c[1:], " "))
	}

	return m
}

// fakeConfirm records the questions and answers them.
type fakeConfirm struct {
	answer bool
	forced []bool
}

func (f *fakeConfirm) ask(force bool, _ string) (bool, error) {
	f.forced = append(f.forced, force)

	return force || f.answer, nil
}

// pruneDistro is a distribution folder with the defaults the merger reads and templates/plugins from testdata.
func pruneDistro(t *testing.T, withPruneScript bool) string {
	t.Helper()

	distro := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(distro, "defaults"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(distro, "defaults", "kfddistribution-kfd-v1alpha2.yaml"), []byte("data: {}\n"), 0o600))
	require.NoError(t, os.CopyFS(filepath.Join(distro, "templates", "plugins"), os.DirFS(filepath.Join("testdata", "plugins"))))

	if !withPruneScript {
		require.NoError(t, os.Remove(filepath.Join(distro, "templates", "plugins", "scripts", "prune.sh.tpl")))
	}

	return distro
}

type pruneCase struct {
	prune   *PluginsPrune
	runner  *fakeScriptRunner
	confirm *fakeConfirm
	paths   cluster.CreatorPaths
}

func newPruneCase(t *testing.T, removed []RemovedPlugin, dryRun bool, force []string, withPruneScript bool) *pruneCase {
	t.Helper()

	work := t.TempDir()
	cfgPath := filepath.Join(work, "furyctl.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("spec: {distribution: {}}\n"), 0o600))

	paths := cluster.CreatorPaths{
		ConfigPath: cfgPath,
		WorkDir:    work,
		DistroPath: pruneDistro(t, withPruneScript),
		BinPath:    filepath.Join(work, "bin"),
	}

	c := &pruneCase{runner: &fakeScriptRunner{fail: map[string]error{}}, confirm: &fakeConfirm{}, paths: paths}
	c.prune = NewPluginsPrune(paths, config.KFD{}, "KFDDistribution", dryRun, force,
		&fakeConfigStore{cfg: []byte(storedWithRelease)}, removed)
	c.prune.shellRunner = c.runner
	c.prune.confirm = c.confirm.ask

	return c
}

var removedX = []RemovedPlugin{helmRelease("n", "x"), kustomize("my-plugin")}

func TestPluginsPruneConfirmation(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc       string
		answer     bool
		force      []string
		dryRun     bool
		wantModes  []string
		wantForced []bool
		wantErr    error
	}{
		{
			desc:       "the user answers no",
			wantModes:  []string{"plan helm:n/x kustomize:my-plugin"},
			wantForced: []bool{false},
			wantErr:    ErrPruneCanceled,
		},
		{
			desc:       "the user answers yes",
			answer:     true,
			wantModes:  []string{"plan helm:n/x kustomize:my-plugin", "delete helm:n/x kustomize:my-plugin"},
			wantForced: []bool{false},
		},
		{
			desc:       "force all skips the question",
			force:      []string{cluster.ForceFeatureAll},
			wantModes:  []string{"plan helm:n/x kustomize:my-plugin", "delete helm:n/x kustomize:my-plugin"},
			wantForced: []bool{true},
		},
		{
			desc:       "force migrations still asks",
			force:      []string{cluster.ForceFeatureMigrations},
			wantModes:  []string{"plan helm:n/x kustomize:my-plugin"},
			wantForced: []bool{false},
			wantErr:    ErrPruneCanceled,
		},
		{
			desc:      "dry run only lists",
			dryRun:    true,
			force:     []string{cluster.ForceFeatureAll},
			wantModes: []string{"plan helm:n/x kustomize:my-plugin"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()

			c := newPruneCase(t, removedX, tc.dryRun, tc.force, true)
			c.confirm.answer = tc.answer

			err := c.prune.Exec()

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tc.wantModes, c.runner.modes())
			assert.Equal(t, tc.wantForced, c.confirm.forced)

			for _, call := range c.runner.calls {
				assert.Equal(t, filepath.Join(c.paths.WorkDir, OperationPhasePluginsPrune, "scripts", "prune.sh"), call[0])
			}
		})
	}
}

func TestPluginsPruneNothingToPrune(t *testing.T) {
	t.Parallel()

	c := newPruneCase(t, nil, false, nil, true)

	require.NoError(t, c.prune.Exec())
	assert.Empty(t, c.runner.calls)
	assert.Empty(t, c.confirm.forced)
}

func TestPluginsPruneOldDistribution(t *testing.T) {
	t.Parallel()

	c := newPruneCase(t, removedX, false, []string{cluster.ForceFeatureAll}, false)

	err := c.prune.Exec()

	require.ErrorIs(t, err, ErrPruneScriptMissing)
	assert.Contains(t, err.Error(), "prune.sh.tpl")
	assert.Contains(t, err.Error(), "--force plugins-unmanage")
	assert.Empty(t, c.runner.calls)
}

func TestPluginsPruneScriptFails(t *testing.T) {
	t.Parallel()

	errGuard := errors.New("apps/data-0 (StorageClass fast)")

	for _, mode := range []string{"plan", "delete"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()

			c := newPruneCase(t, removedX, false, []string{cluster.ForceFeatureAll}, true)
			c.runner.fail[mode] = errGuard

			err := c.prune.Exec()

			require.ErrorIs(t, err, errGuard)
			assert.Contains(t, err.Error(), "apps/data-0 (StorageClass fast)")
		})
	}
}

func TestPluginsPruneRendersTheStoredConfiguration(t *testing.T) {
	t.Parallel()

	c := newPruneCase(t, removedX, false, []string{cluster.ForceFeatureAll}, true)

	require.NoError(t, c.prune.Exec())

	helmfile, err := os.ReadFile(filepath.Join(c.paths.WorkDir, OperationPhasePluginsPrune, "helmfile.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(helmfile), "name: x")
	assert.Contains(t, string(helmfile), "chart: c")

	assert.NoDirExists(t, filepath.Join(c.paths.WorkDir, cluster.OperationPhasePlugins))
}

func TestPluginsExecPrunesFirst(t *testing.T) {
	t.Parallel()

	c := newPruneCase(t, removedX, false, []string{cluster.ForceFeatureAll}, true)
	c.runner.fail["delete"] = errFake

	plugins := NewPlugins(c.paths, config.KFD{}, "KFDDistribution", false)
	plugins.PruneFirst(c.prune)

	require.ErrorIs(t, plugins.Exec(), errFake)

	// The failed prune stops the plugins phase before it renders anything.
	assert.NoDirExists(t, plugins.Path)
	assert.False(t, plugins.Applied())
}
