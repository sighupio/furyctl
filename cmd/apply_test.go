// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package cmd

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/cluster"
)

func TestPhasesReadPKI(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc            string
		phase           string
		startFrom       string
		postApplyPhases []string
		want            bool
	}{
		{
			desc:  "all the phases",
			phase: cluster.OperationPhaseAll,
			want:  true,
		},
		{
			desc:  "the infrastructure phase alone",
			phase: cluster.OperationPhaseInfrastructure,
			want:  true,
		},
		{
			desc:  "the kubernetes phase alone",
			phase: cluster.OperationPhaseKubernetes,
			want:  true,
		},
		{
			desc:  "the distribution phase alone",
			phase: cluster.OperationPhaseDistribution,
			want:  false,
		},
		{
			desc:  "the plugins phase alone",
			phase: cluster.OperationPhasePlugins,
			want:  false,
		},
		{
			desc:      "all the phases from the kubernetes phase",
			phase:     cluster.OperationPhaseAll,
			startFrom: cluster.OperationPhaseKubernetes,
			want:      true,
		},
		{
			desc:      "all the phases from the distribution phase",
			phase:     cluster.OperationPhaseAll,
			startFrom: cluster.OperationPhaseDistribution,
			want:      false,
		},
		{
			desc:      "all the phases from the pre-distribution phase",
			phase:     cluster.OperationPhaseAll,
			startFrom: cluster.OperationSubPhasePreDistribution,
			want:      false,
		},
		{
			desc:      "all the phases from the post-distribution phase",
			phase:     cluster.OperationPhaseAll,
			startFrom: cluster.OperationSubPhasePostDistribution,
			want:      false,
		},
		{
			desc:      "all the phases from the plugins phase",
			phase:     cluster.OperationPhaseAll,
			startFrom: cluster.OperationPhasePlugins,
			want:      false,
		},
		{
			// coreKubernetes skips apply.yaml, the playbook that copies the CA files.
			desc:      "all the phases from the post-kubernetes phase",
			phase:     cluster.OperationPhaseAll,
			startFrom: cluster.OperationSubPhasePostKubernetes,
			want:      false,
		},
		{
			desc:      "all the phases from the pre-kubernetes phase",
			phase:     cluster.OperationPhaseAll,
			startFrom: cluster.OperationSubPhasePreKubernetes,
			want:      true,
		},
		{
			// The extra phases run after the apply, so the kubernetes phase runs even when startFrom
			// excluded it. See ClusterCreator.extraPhases.
			desc:            "from the distribution phase, with the kubernetes phase after the apply",
			phase:           cluster.OperationPhaseAll,
			startFrom:       cluster.OperationPhaseDistribution,
			postApplyPhases: []string{cluster.OperationPhaseKubernetes},
			want:            true,
		},
		{
			// extraPhases ignores an infrastructure value, so no phase reads the PKI here.
			desc:            "from the distribution phase, with the infrastructure phase after the apply",
			phase:           cluster.OperationPhaseAll,
			startFrom:       cluster.OperationPhaseDistribution,
			postApplyPhases: []string{cluster.OperationPhaseInfrastructure},
			want:            false,
		},
		{
			desc:            "from the distribution phase, with the distribution phase after the apply",
			phase:           cluster.OperationPhaseAll,
			startFrom:       cluster.OperationPhaseDistribution,
			postApplyPhases: []string{cluster.OperationPhaseDistribution},
			want:            false,
		},
		{
			desc:            "from the plugins phase, with the plugins phase after the apply",
			phase:           cluster.OperationPhaseAll,
			startFrom:       cluster.OperationPhasePlugins,
			postApplyPhases: []string{cluster.OperationPhasePlugins},
			want:            false,
		},
		{
			desc:            "all the phases, with the distribution phase after the apply",
			phase:           cluster.OperationPhaseAll,
			postApplyPhases: []string{cluster.OperationPhaseDistribution},
			want:            true,
		},
	}

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tC.want, phasesReadPKI(tC.phase, tC.startFrom, tC.postApplyPhases))
		})
	}
}

func TestApplyForceFlagListsPluginsUnmanage(t *testing.T) {
	t.Parallel()

	applyCmd := NewApplyCmd()

	assert.Contains(t, applyCmd.Flags().Lookup("force").Usage, cluster.ForceFeaturePluginsUnmanage)

	rootCmd := &cobra.Command{Use: "furyctl"}
	rootCmd.AddCommand(applyCmd)

	out := &bytes.Buffer{}
	rootCmd.SetOut(out)
	rootCmd.SetArgs([]string{cobra.ShellCompRequestCmd, "apply", "--force", ""})

	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, out.String(), cluster.ForceFeaturePluginsUnmanage+"\n")
}

func TestApplyHelpListsPrunePlugins(t *testing.T) {
	t.Parallel()

	flag := NewApplyCmd().Flags().Lookup("prune-plugins")

	require.NotNil(t, flag)
	assert.Contains(t, flag.Usage, "removed from spec.plugins")
}

func TestValidatePrunePluginsFlag(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc     string
		phase    string
		force    []string
		wantText []string
	}{
		{desc: "all the phases", phase: cluster.OperationPhaseAll},
		{desc: "the plugins phase", phase: cluster.OperationPhasePlugins},
		{desc: "force all", force: []string{cluster.ForceFeatureAll}},
		{
			desc: "another phase", phase: cluster.OperationPhaseDistribution,
			wantText: []string{"--prune-plugins", "--phase plugins", "--phase distribution"},
		},
		{
			desc: "force plugins-unmanage", force: []string{cluster.ForceFeaturePluginsUnmanage},
			wantText: []string{"--prune-plugins", "--force plugins-unmanage"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()

			err := validatePrunePluginsFlag(tc.phase, tc.force)

			if len(tc.wantText) == 0 {
				require.NoError(t, err)

				return
			}

			require.ErrorIs(t, err, ErrPrunePluginsUsage)

			for _, text := range tc.wantText {
				assert.Contains(t, err.Error(), text)
			}
		})
	}
}
