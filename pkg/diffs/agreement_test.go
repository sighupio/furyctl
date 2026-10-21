// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package diffs_test

import (
	"slices"
	"testing"

	r3diff "github.com/r3labs/diff/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/pkg/diffs"
	rules "github.com/sighupio/furyctl/pkg/rulesextractor"
)

// applyUnsafeReducers repeats the extractor calls each creator makes to build its
// unsafe reducer lists, with the same phases, expansion and order. The creators
// compute them inline, so there is no function to call. The lines are the ones of
// each creator before OnPremises and Immutable took their lists from the shared verdict.
func applyUnsafeReducers(t *testing.T, kind string, r rules.Extractor, changelog r3diff.Changelog) []rules.Rule {
	t.Helper()

	switch kind {
	case kindOnPremises:
		// As in internal/apis/kfd/v1alpha2/onpremises/creator.go.
		unsafeReducers := r.UnsafeReducerRulesByDiffs(r.GetReducers(phaseDistribution), changelog)
		unsafeKubeReducers := r.UnsafeReducerRulesByDiffs(
			r.GetReducers(phaseKubernetes),
			diffs.ExpandMapChanges(changelog),
		)

		return slices.Concat(unsafeReducers, unsafeKubeReducers)

	case kindImmutable:
		// As in internal/apis/kfd/v1alpha2/immutable/creator.go.
		unsafeReducersInfrastructure := r.UnsafeReducerRulesByDiffs(r.GetReducers(phaseInfrastructure), changelog)
		unsafeReducersDistribution := r.UnsafeReducerRulesByDiffs(r.GetReducers(phaseDistribution), changelog)

		return slices.Concat(unsafeReducersInfrastructure, unsafeReducersDistribution)

	case kindEKSCluster:
		// As in internal/apis/kfd/v1alpha2/ekscluster/creator.go.
		return r.UnsafeReducerRulesByDiffs(r.GetReducers(phaseDistribution), changelog)

	case kindKFDDistribution:
		// As in internal/apis/kfd/v1alpha2/kfddistribution/creator.go.
		return r.UnsafeReducerRulesByDiffs(r.GetReducers(phaseDistribution), changelog)

	default:
		t.Fatalf("unknown kind %q", kind)

		return nil
	}
}

func changesWithUnsafeReducers(verdicts []diffs.ChangeVerdict) []string {
	paths := []string{}

	for _, v := range verdicts {
		for _, unsafe := range v.UnsafeReducers {
			if len(unsafe) > 0 {
				paths = append(paths, v.Path())

				break
			}
		}
	}

	return paths
}

// unsafeReducersOfVerdict joins the unsafe reducer lists of the decisions in the order
// of the phases of the profile, as the creators join theirs.
func unsafeReducersOfVerdict(t *testing.T, kind string, decisions diffs.RuleDecisions) []rules.Rule {
	t.Helper()

	profile, ok := diffs.ProfileForKind(kind)
	require.True(t, ok, kind)

	var unsafe []rules.Rule

	for _, phase := range profile.UnsafeReducerPhases {
		unsafe = append(unsafe, decisions.UnsafeReducers[phase]...)
	}

	return unsafe
}

func TestVerdicts_AgreeWithTheUnsafeReducersOfApply(t *testing.T) {
	t.Parallel()

	allKinds := []string{kindOnPremises, kindImmutable, kindEKSCluster, kindKFDDistribution}

	testCases := []struct {
		name  string
		kinds []string
		// The one field that changes.
		change field
		// The paths of the rules with unsafe reducers, in the order apply lists them,
		// and the leaf changes the verdict marks with them.
		wantRules   []string
		wantChanges []string
	}{
		{
			name:        "an unsafe reducer",
			kinds:       allKinds,
			change:      field{".spec.distribution.modules.monitoring.type", "prometheus", "none"},
			wantRules:   []string{".spec.distribution.modules.monitoring.type"},
			wantChanges: []string{".spec.distribution.modules.monitoring.type"},
		},
		{
			name:        "a safe reducer",
			kinds:       allKinds,
			change:      field{pathLoggingType, "none", "loki"},
			wantRules:   []string{},
			wantChanges: []string{},
		},
		{
			name:        "a change without reducer",
			kinds:       allKinds,
			change:      field{pathRetentionTime, nil, "15d"},
			wantRules:   []string{},
			wantChanges: []string{},
		},
		{
			name:        "an unsafe reducer of a leaf inside an added object where apply expands",
			kinds:       []string{kindOnPremises},
			change:      kubeProxyAdded("nftables"),
			wantRules:   []string{pathKubeProxyType},
			wantChanges: []string{pathKubeProxyType},
		},
		{
			name:        "a leaf inside an added object where apply does not expand",
			kinds:       []string{kindImmutable, kindEKSCluster, kindKFDDistribution},
			change:      kubeProxyAdded("nftables"),
			wantRules:   []string{},
			wantChanges: []string{},
		},
		{
			name:        "an unsafe reducer of the infrastructure phase",
			kinds:       []string{kindImmutable},
			change:      nodeRenamed(),
			wantRules:   []string{".spec.infrastructure.nodes.*.hostname"},
			wantChanges: []string{".spec.infrastructure.nodes.0.hostname"},
		},
		{
			name:        "an infrastructure change where apply reads no infrastructure reducer",
			kinds:       []string{kindOnPremises, kindEKSCluster, kindKFDDistribution},
			change:      nodeRenamed(),
			wantRules:   []string{},
			wantChanges: []string{},
		},
	}

	for _, tC := range testCases {
		for _, kind := range tC.kinds {
			t.Run(tC.name+" in "+kind, func(t *testing.T) {
				t.Parallel()

				current, updated := configs(tC.change)

				changelog, err := diffs.NewBaseChecker(current, updated).GenerateDiff()
				require.NoError(t, err)

				// The creator gives the extractor the new configuration.
				applyUnsafe := applyUnsafeReducers(t, kind, extractorFor(t, kind, realRules, updated), changelog)

				assert.Equal(t, tC.wantRules, rules.Paths(applyUnsafe), "unsafe reducers of apply")

				// The diff gives the extractor the stored configuration. The two sides
				// agree on these changes because no safe condition of their rules reads
				// a field that differs between the two configurations.
				profile, ok := diffs.ProfileForKind(kind)
				require.True(t, ok)

				decisions := diffs.DecideRules(changelog, extractorFor(t, kind, realRules, current), profile)

				assert.Equal(
					t,
					tC.wantRules,
					rules.Paths(unsafeReducersOfVerdict(t, kind, decisions)),
					"unsafe reducers of the verdict",
				)
				assert.Equal(t, tC.wantChanges, changesWithUnsafeReducers(decisions.Verdicts))
			})
		}
	}
}
