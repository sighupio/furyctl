// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package cluster_test

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	r3diff "github.com/r3labs/diff/v3"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/cluster"
	osx "github.com/sighupio/furyctl/internal/x/os"
	templatex "github.com/sighupio/furyctl/pkg/template"
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

// TestCopyFromTemplate_RemovesTempDir checks that CopyFromTemplate removes the folder that holds the
// rendered configuration.
func TestCopyFromTemplate_RemovesTempDir(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("TMPDIR", tmpDir)

	srcDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "out.txt.tpl"), []byte("{{ .spec.name }}"), 0o644))

	targetDir := t.TempDir()
	cfg := templatex.Config{Data: map[string]map[any]any{"spec": {"name": "test"}}}

	err := (&cluster.OperationPhase{}).CopyFromTemplate(cfg, "test", srcDir, targetDir, "")
	require.NoError(t, err)

	out, err := os.ReadFile(filepath.Join(targetDir, "out.txt"))
	require.NoError(t, err)
	require.Equal(t, "test", string(out))

	left, err := filepath.Glob(filepath.Join(tmpDir, "furyctl-test-*"))
	require.NoError(t, err)
	require.Empty(t, left)
}

// TestCopyFromTemplate_RegistersTempDir checks that the folder of the rendered configuration is registered for
// removal, so that the signal handler removes it when a signal stops furyctl before the deferred removal runs.
func TestCopyFromTemplate_RegistersTempDir(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("TMPDIR", tmpDir)

	logs := &bytes.Buffer{}
	level, output := logrus.GetLevel(), logrus.StandardLogger().Out

	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetOutput(logs)

	t.Cleanup(func() {
		logrus.SetLevel(level)
		logrus.SetOutput(output)
	})

	srcDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "out.txt.tpl"), []byte("{{ .spec.name }}"), 0o644))

	cfg := templatex.Config{Data: map[string]map[any]any{"spec": {"name": "test"}}}

	require.NoError(t, (&cluster.OperationPhase{}).CopyFromTemplate(cfg, "registered", srcDir, t.TempDir(), ""))

	match := regexp.MustCompile(`registered configuration file path ([^\s"]+)`).FindStringSubmatch(logs.String())
	require.Len(t, match, 2)

	// The deferred removal ran. A folder at the same path, as a signal would leave it, goes at exit.
	outDir := filepath.Dir(match[1])
	require.Equal(t, tmpDir, filepath.Dir(outDir))
	require.NoDirExists(t, outDir)
	require.NoError(t, os.Mkdir(outDir, 0o700))

	osx.RemoveTempDirs()

	require.NoDirExists(t, outDir)
}

func TestOperationPhase_CopyKFDToConfig(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc    string
		kfd     *string
		want    map[any]any
		wantErr bool
	}{
		{
			desc: "exposes version and modules with the keys of the file",
			kfd: new(`version: v1.36.0
modules:
  monitoring: v4.3.0
  utilities: v0.1.1
  futuremodule: v0.0.1
`),
			want: map[any]any{
				"version": "v1.36.0",
				"modules": map[any]any{
					"monitoring":   "v4.3.0",
					"utilities":    "v0.1.1",
					"futuremodule": "v0.0.1",
				},
			},
		},
		{
			desc:    "fails when kfd.yaml is missing",
			kfd:     nil,
			wantErr: true,
		},
		{
			desc:    "fails when kfd.yaml is not valid yaml",
			kfd:     new("version: [v1.36.0"),
			wantErr: true,
		},
	}

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			distroPath := t.TempDir()

			if tC.kfd != nil {
				require.NoError(t, os.WriteFile(filepath.Join(distroPath, "kfd.yaml"), []byte(*tC.kfd), 0o600))
			}

			cfg := templatex.Config{Data: map[string]map[any]any{
				"checks": {"storageClassAvailable": true},
			}}

			err := (&cluster.OperationPhase{}).CopyKFDToConfig(&cfg, distroPath)

			if tC.wantErr {
				require.Error(t, err)
				require.NotContains(t, cfg.Data, "kfd")

				return
			}

			require.NoError(t, err)
			require.Equal(t, tC.want, cfg.Data["kfd"])
			require.Equal(t, map[any]any{"storageClassAvailable": true}, cfg.Data["checks"])
		})
	}
}
