// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package diffs_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/pkg/diffs"
	rules "github.com/sighupio/furyctl/pkg/rulesextractor"
)

// syntheticProfileRules is the folder that holds rules/<kind>-kfd-v1alpha2.yaml written
// for the tests of the profiles, not taken from the distribution.
const syntheticProfileRules = "testdata/synthetic"

func TestRequireProfileForKind(t *testing.T) {
	t.Parallel()

	t.Run("every kind of the configuration has a profile", func(t *testing.T) {
		t.Parallel()

		for _, kind := range distribution.ConfigKinds() {
			got, err := diffs.RequireProfileForKind(kind)
			require.NoError(t, err, kind)

			want, ok := diffs.ProfileForKind(kind)
			require.True(t, ok, kind)
			assert.Equal(t, want, got, kind)
		}
	})

	t.Run("a kind without a profile is an error, not a profile with no check", func(t *testing.T) {
		t.Parallel()

		for _, kind := range []string{"Immutabel", "onpremises", ""} {
			got, err := diffs.RequireProfileForKind(kind)

			require.ErrorIs(t, err, diffs.ErrNoKindProfile, kind)
			assert.Equal(t, diffs.KindProfile{}, got, kind)
		}
	})
}

func TestVerdicts_SafeReducerRuleMatchingSeveralChanges(t *testing.T) {
	t.Parallel()

	distroPath := syntheticRules(t, `infrastructure:
  - path: .spec.infrastructure.nodes.*.arch
    immutable: false
    description: "the architecture of a node changed"
    safe:
      - from: none
    reducers:
      - key: infrastructureNodesArch
        lifecycle: pre-apply
`)

	nodes := func(first, second string) map[string]any {
		return map[string]any{"spec": map[string]any{"infrastructure": map[string]any{"nodes": []any{
			map[string]any{"arch": first}, map[string]any{"arch": second},
		}}}}
	}

	current, updated := nodes("none", "none"), nodes("x86", "arm")

	changelog, err := diffs.NewBaseChecker(current, updated).GenerateDiff()
	require.NoError(t, err)
	require.Len(t, changelog, 2)

	profile, ok := diffs.ProfileForKind(kindImmutable)
	require.True(t, ok)

	verdicts := diffs.Verdicts(changelog, extractorFor(t, kindImmutable, distroPath, current), profile)
	require.Len(t, verdicts, 2)

	// The one rule matches both changes: each of them is a migration, not only the first.
	for _, v := range verdicts {
		assert.Equal(t, diffs.VerdictMigration, v.Verdict, v.Path())
		assert.Equal(t, "the architecture of a node changed", v.Text, v.Path())
	}
}

func TestVerdicts_EKSClusterProfile(t *testing.T) {
	t.Parallel()

	const (
		pathVPCCIDR     = ".spec.infrastructure.vpc.network.cidr"
		pathVPN         = ".spec.infrastructure.vpn.instances"
		pathServiceCIDR = ".spec.kubernetes.serviceIpV4Cidr"
		pathLaunchKind  = ".spec.kubernetes.nodePoolsLaunchKind"
	)

	changes := []field{
		{pathVPCCIDR, "10.0.0.0/16", "10.1.0.0/16"},
		{pathVPN, 2, 1},
		{pathServiceCIDR, "172.20.0.0/16", "172.21.0.0/16"},
		{pathLaunchKind, "launch_templates", "launch_configurations"},
	}

	current, updated := configs(changes...)

	changelog, err := diffs.NewBaseChecker(current, updated).GenerateDiff()
	require.NoError(t, err)

	extractor := extractorFor(t, kindEKSCluster, syntheticProfileRules, current)

	t.Run("the immutable and unsupported rules of infrastructure and kubernetes block", func(t *testing.T) {
		t.Parallel()

		profile, ok := diffs.ProfileForKind(kindEKSCluster)
		require.True(t, ok)

		decisions := diffs.DecideRules(changelog, extractor, profile)

		wantText := map[string]string{
			pathVPCCIDR:     "the CIDR of the VPC cannot be changed",
			pathVPN:         "removing a VPN instance is not supported",
			pathServiceCIDR: "the service CIDR cannot be changed",
			pathLaunchKind:  "going back to launch configurations is not supported",
		}

		require.Len(t, decisions.Verdicts, len(wantText))

		for path, text := range wantText {
			v := verdictAt(t, decisions.Verdicts, path)

			assert.Equal(t, diffs.VerdictBlocked, v.Verdict, path)
			assert.Equal(t, text, v.Text, path)
		}

		// The errors of the two checks of the preflight, phase after phase.
		assert.Equal(
			t,
			[]string{
				"immutable value changed: path " + pathVPCCIDR + "  oldValue 10.0.0.0/16 newValue 10.1.0.0/16",
				"immutable value changed: path " + pathServiceCIDR + "  oldValue 172.20.0.0/16 newValue 172.21.0.0/16",
			},
			errorTexts(decisions.ImmutableViolations),
		)
		assert.Equal(
			t,
			[]string{
				"unsupported value changed: removing a VPN instance is not supported",
				"unsupported value changed: going back to launch configurations is not supported",
			},
			errorTexts(decisions.UnsupportedViolations),
		)
	})

	t.Run("a profile that checks the distribution phase only lets them through", func(t *testing.T) {
		t.Parallel()

		profile, ok := diffs.ProfileForKind(kindKFDDistribution)
		require.True(t, ok)

		decisions := diffs.DecideRules(changelog, extractor, profile)

		for _, v := range decisions.Verdicts {
			assert.Equal(t, diffs.VerdictNone, v.Verdict, v.Path())
		}

		assert.Empty(t, decisions.ImmutableViolations)
		assert.Empty(t, decisions.UnsupportedViolations)
	})
}

func TestVerdicts_OnPremisesProfile(t *testing.T) {
	t.Parallel()

	const (
		pathSchedulerProfile    = ".spec.kubernetes.advanced.scheduler.profile"
		pathEncryption          = ".spec.kubernetes.advanced.encryption"
		pathEncryptionProvider  = pathEncryption + ".provider"
		pathEncryptionMode      = pathEncryption + ".mode"
		textKubernetesProfile   = "kubernetes phase: the scheduler profile changed"
		textDistributionProfile = "distribution phase: the scheduler profile changed"
	)

	profile, ok := diffs.ProfileForKind(kindOnPremises)
	require.True(t, ok)

	decide := func(t *testing.T, profile diffs.KindProfile, change field) diffs.RuleDecisions {
		t.Helper()

		current, updated := configs(change)

		changelog, err := diffs.NewBaseChecker(current, updated).GenerateDiff()
		require.NoError(t, err)

		return diffs.DecideRules(changelog, extractorFor(t, kindOnPremises, syntheticProfileRules, current), profile)
	}

	t.Run("the distribution reducers are checked before the kubernetes ones", func(t *testing.T) {
		t.Parallel()

		got := decide(t, profile, field{pathSchedulerProfile, "default", "strict"})

		require.Len(t, got.Verdicts, 1)
		assert.Equal(t, diffs.VerdictNeedsConfirmation, got.Verdicts[0].Verdict)
		assert.Equal(t, textDistributionProfile, got.Verdicts[0].Text)
		assert.Equal(t, []string{pathSchedulerProfile}, rules.Paths(got.UnsafeReducers[phaseDistribution]))
		assert.Equal(t, []string{pathSchedulerProfile}, rules.Paths(got.UnsafeReducers[phaseKubernetes]))

		// The order is what decides the text: in the other order the kubernetes rule
		// gives it.
		swapped := profile
		swapped.UnsafeReducerPhases = []string{phaseKubernetes, phaseDistribution}

		got = decide(t, swapped, field{pathSchedulerProfile, "default", "strict"})

		assert.Equal(t, textKubernetesProfile, got.Verdicts[0].Text)
	})

	t.Run("the kubernetes reducers reach the leaves of an added object, the distribution ones do not", func(t *testing.T) {
		t.Parallel()

		got := decide(t, profile, field{pathEncryption, nil, map[string]any{"provider": "aescbc", "mode": "full"}})

		require.Len(t, got.Verdicts, 2)

		provider := verdictAt(t, got.Verdicts, pathEncryptionProvider)
		assert.Equal(t, diffs.VerdictNeedsConfirmation, provider.Verdict)
		assert.Equal(t, "kubernetes phase: the encryption provider changed", provider.Text)

		assert.Equal(t, diffs.VerdictNone, verdictAt(t, got.Verdicts, pathEncryptionMode).Verdict)

		assert.Equal(t, []string{pathEncryptionProvider}, rules.Paths(got.UnsafeReducers[phaseKubernetes]))
		assert.Empty(t, got.UnsafeReducers[phaseDistribution])

		// Without the expansion the kubernetes rule does not reach the leaf either.
		reported := profile
		reported.ExpandedReducerPhases = nil

		got = decide(t, reported, field{pathEncryption, nil, map[string]any{"provider": "aescbc", "mode": "full"}})

		assert.Equal(t, diffs.VerdictNone, verdictAt(t, got.Verdicts, pathEncryptionProvider).Verdict)
		assert.Empty(t, got.UnsafeReducers[phaseKubernetes])
	})
}
