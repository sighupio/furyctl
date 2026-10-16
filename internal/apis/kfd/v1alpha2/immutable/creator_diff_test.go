// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package immutable_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/apis/config"
	commcreate "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/common/create"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/immutable"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/immutable/create"
	"github.com/sighupio/furyctl/internal/cluster"
)

func newDiffCreator(t *testing.T, phase string) *immutable.ClusterCreator {
	t.Helper()

	root := t.TempDir()
	distro := filepath.Join(root, "distro")
	configPath := filepath.Join(root, "furyctl.yaml")

	require.NoError(t, os.MkdirAll(filepath.Join(distro, "defaults"), os.FileMode(0o755)))
	require.NoError(t, os.WriteFile(filepath.Join(distro, "defaults", "immutable-kfd-v1alpha2.yaml"), []byte("data: {}\n"), os.FileMode(0o600)))
	require.NoError(t, os.WriteFile(configPath, []byte(
		"apiVersion: kfd.sighup.io/v1alpha2\nkind: Immutable\nspec:\n  distribution: {}\n  plugins:\n"+
			"    kustomize:\n      - name: alpha\n        folder: ./alpha\n"+
			"    helm:\n      releases:\n        - name: poc-podinfo\n          chart: podinfo/podinfo\n",
	), os.FileMode(0o600)))

	c := &immutable.ClusterCreator{}

	c.SetProperty(cluster.CreatorPropertyConfigPath, configPath)
	c.SetProperty(cluster.CreatorPropertyDistroPath, distro)
	c.SetProperty(cluster.CreatorPropertyWorkDir, filepath.Join(root, "work"))
	c.SetProperty(cluster.CreatorPropertyBinPath, filepath.Join(root, "bin"))
	c.SetProperty(cluster.CreatorPropertyKfdManifest, config.KFD{Version: "v1.36.0"})
	c.SetProperty(cluster.CreatorPropertyPhase, phase)

	return c
}

func TestDiff_PhasesWithoutPlan(t *testing.T) {
	t.Parallel()

	for _, phase := range []string{"infrastructure", "kubernetes"} {
		t.Run(phase, func(t *testing.T) {
			t.Parallel()

			c := newDiffCreator(t, phase)

			sections, err := c.Diff(func() {})

			require.NoError(t, err)
			require.Equal(t, []commcreate.Section{
				{Title: "Distribution", Status: commcreate.StatusOutOfScope, Reason: commcreate.ReasonNoPlanForPhase},
				{Title: "Plugins (kustomize)", Status: commcreate.StatusOutOfScope, Reason: commcreate.ReasonNoPlanForPhase},
				{Title: "Plugins (helm)", Status: commcreate.StatusOutOfScope, Reason: commcreate.ReasonNoPlanForPhase},
			}, sections)
		})
	}
}

func TestDiff_UnsupportedPhase(t *testing.T) {
	t.Parallel()

	c := newDiffCreator(t, "bogus")

	sections, err := c.Diff(func() {})

	require.ErrorIs(t, err, immutable.ErrUnsupportedPhase)
	require.Nil(t, sections)
}

type recordingPreflight struct {
	calls *[]string
}

func (r recordingPreflight) Exec(map[string]any) (*create.Status, error) {
	*r.calls = append(*r.calls, "preflight Exec")

	return nil, errBoom
}

var errBoom = errors.New("boom")

func TestDiff_PreflightGetsTheApplyDefaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		force   []string
		upgrade bool
		dryRun  bool
	}{
		{"creator with force all, upgrade and dry-run", []string{"all"}, true, true},
		{"creator with the apply defaults", nil, false, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			c := newDiffCreator(t, "")
			c.SetProperty(cluster.CreatorPropertyForce, tc.force)
			c.SetProperty(cluster.CreatorPropertyUpgrade, tc.upgrade)
			c.SetProperty(cluster.CreatorPropertyDryRun, tc.dryRun)

			calls := []string{}
			c.SetNewPreflight(func(dryRun bool, force []string, startFrom string, upgrade bool) immutable.DiffPreflight {
				calls = append(calls, fmt.Sprintf("preflight built with force %v, start-from %q, upgrade %t, dry-run %t",
					force, startFrom, upgrade, dryRun))

				return recordingPreflight{calls: &calls}
			})

			sections, err := c.Diff(func() {})

			require.NoError(t, err)
			require.Equal(t, []string{
				`preflight built with force [], start-from "", upgrade false, dry-run false`,
				"preflight Exec",
			}, calls)
			require.Equal(t, []commcreate.Section{
				{Title: "Distribution", Status: commcreate.StatusFailed, Reason: "preflight failed: boom"},
				{Title: "Plugins (kustomize)", Status: commcreate.StatusFailed, Reason: "preflight failed: boom"},
				{Title: "Plugins (helm)", Status: commcreate.StatusFailed, Reason: "preflight failed: boom"},
			}, sections)
		})
	}
}

type outcomePreflight struct {
	calls  *[]string
	status *create.Status
	err    error
}

func (o outcomePreflight) Exec(map[string]any) (*create.Status, error) {
	*o.calls = append(*o.calls, "preflight Exec")

	return o.status, o.err
}

// diff-command-06: afterPreflight runs once, right after the preflight Exec, whatever its outcome.
func TestDiff_AfterPreflightRunsOnceRightAfterThePreflightExec(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		status *create.Status
		err    error
	}{
		"preflight succeeds":         {&create.Status{Success: true, ClusterExists: true}, nil},
		"preflight fails":            {nil, errBoom},
		"preflight finds no cluster": {&create.Status{}, nil},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			c := newDiffCreator(t, "")

			calls := []string{}
			c.SetNewPreflight(func(bool, []string, string, bool) immutable.DiffPreflight {
				return outcomePreflight{calls: &calls, status: tc.status, err: tc.err}
			})

			_, err := c.Diff(func() { calls = append(calls, "afterPreflight") })

			require.NoError(t, err)
			require.Equal(t, []string{"preflight Exec", "afterPreflight"}, calls)
		})
	}
}
