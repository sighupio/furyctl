// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package immutable //nolint:testpackage // shares its cases with the test of the unexported unsafe reducer lists.

import (
	"strings"
	"testing"

	r3diff "github.com/r3labs/diff/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/immutable/supported"
	"github.com/sighupio/furyctl/internal/cluster"
	rules "github.com/sighupio/furyctl/pkg/rulesextractor"
)

// These tests pin the lists of rules with unsafe reducers the creator of Immutable
// computes for each phase, against the real rule files of the distribution. The
// creator computed them inline before it took them from the shared verdict, so the
// test repeats its extractor calls. It is not edited after the creator takes the lists from the shared
// verdict.

// characterizedRules is the folder that holds the real rule files of the distribution
// (pkg/diffs/testdata/rules, see its PROVENANCE).
const characterizedRules = "../../../../../pkg/diffs/testdata"

// charRule is one rule of a list: its path and the values the extractor wrote into
// its first reducer, which is what the creator would read from it.
type charRule struct {
	path string
	from any
	to   any
}

func charRules(list []rules.Rule) []charRule {
	out := []charRule{}

	for _, rule := range list {
		reducer := (*rule.Reducers)[0]

		out = append(out, charRule{path: rule.Path, from: reducer.From, to: reducer.To})
	}

	return out
}

// charChange is one change of a changelog, written with its dotted path.
func charChange(path string, from, to any) r3diff.Change {
	return r3diff.Change{
		Type: r3diff.UPDATE,
		Path: strings.Split(strings.TrimPrefix(path, "."), "."),
		From: from,
		To:   to,
	}
}

// charExtractor builds the extractor as the creator does: it gives it the new
// configuration and goes on without rules when the distribution has no rule file.
func charExtractor(t *testing.T, distroPath string) *rules.ImmutableExtractor {
	t.Helper()

	r, err := rules.NewImmutableClusterRulesExtractor(distroPath, map[string]any{}, supported.Phases())
	if err != nil {
		require.ErrorIs(t, err, rules.ErrReadingRulesFile)
	}

	return r
}

type charUnsafeReducersCase struct {
	name               string
	noRules            bool
	changelog          r3diff.Changelog
	wantInfrastructure []charRule
	wantDistribution   []charRule
}

func charUnsafeReducersCases() []charUnsafeReducersCase {
	return []charUnsafeReducersCase{
		{
			name: "a change without reducer gives empty lists",
			changelog: r3diff.Changelog{
				charChange(".spec.distribution.modules.monitoring.prometheus.retentionTime", nil, "15d"),
			},
			wantInfrastructure: []charRule{},
			wantDistribution:   []charRule{},
		},
		{
			name: "a safe reducer gives empty lists",
			changelog: r3diff.Changelog{
				charChange(".spec.distribution.modules.logging.type", "none", "loki"),
			},
			wantInfrastructure: []charRule{},
			wantDistribution:   []charRule{},
		},
		{
			name: "an unsafe reducer is in the list of its phase",
			changelog: r3diff.Changelog{
				charChange(".spec.distribution.modules.monitoring.type", "prometheus", "none"),
			},
			wantInfrastructure: []charRule{},
			wantDistribution:   []charRule{{".spec.distribution.modules.monitoring.type", "prometheus", "none"}},
		},
		{
			name: "a field added is not the literal value none of a safe condition",
			changelog: r3diff.Changelog{
				charChange(".spec.distribution.modules.logging.type", nil, "loki"),
			},
			wantInfrastructure: []charRule{},
			wantDistribution:   []charRule{{".spec.distribution.modules.logging.type", nil, "loki"}},
		},
		{
			name: "the list follows the order of the rules, not of the changelog",
			changelog: r3diff.Changelog{
				charChange(".spec.distribution.modules.monitoring.type", "prometheus", "none"),
				charChange(".spec.distribution.common.networkPoliciesEnabled", false, true),
			},
			wantInfrastructure: []charRule{},
			wantDistribution:   []charRule{{".spec.distribution.common.networkPoliciesEnabled", false, true}, {".spec.distribution.modules.monitoring.type", "prometheus", "none"}},
		},
		{
			name: "a safe and an unsafe reducer give only the unsafe one",
			changelog: r3diff.Changelog{
				charChange(".spec.distribution.modules.logging.type", "none", "loki"),
				charChange(".spec.distribution.common.networkPoliciesEnabled", false, true),
			},
			wantInfrastructure: []charRule{},
			wantDistribution:   []charRule{{".spec.distribution.common.networkPoliciesEnabled", false, true}},
		},
		{
			name:    "without a rule file the lists are empty",
			noRules: true,
			changelog: r3diff.Changelog{
				charChange(".spec.distribution.modules.monitoring.type", "prometheus", "none"),
				charChange(".spec.distribution.common.networkPoliciesEnabled", false, true),
			},
			wantInfrastructure: []charRule{},
			wantDistribution:   []charRule{},
		},
		{
			name: "a rule matched by two changes is twice in the list, with the values of the last one",
			changelog: r3diff.Changelog{
				charChange(".spec.infrastructure.nodes.0.hostname", "node-a", "node-b"),
				charChange(".spec.infrastructure.nodes.1.hostname", "node-c", "node-d"),
			},
			wantInfrastructure: []charRule{{".spec.infrastructure.nodes.*.hostname", "node-c", "node-d"}, {".spec.infrastructure.nodes.*.hostname", "node-c", "node-d"}},
			wantDistribution:   []charRule{},
		},
		{
			name: "a whole object added under a recursive path is once in the list",
			changelog: r3diff.Changelog{
				charChange(".spec.infrastructure.nodes.0.storage.disks", nil, map[string]any{"device": "/dev/sda", "wipeTable": true}),
			},
			wantInfrastructure: []charRule{{".spec.infrastructure.nodes.*.storage.**", nil, map[string]any{"device": "/dev/sda", "wipeTable": true}}},
			wantDistribution:   []charRule{},
		},
		{
			name: "a leaf inside a whole object added reaches no kubernetes reducer",
			changelog: r3diff.Changelog{
				charChange(".spec.kubernetes.advanced.kubeProxy", nil, map[string]any{"type": "nftables"}),
			},
			wantInfrastructure: []charRule{},
			wantDistribution:   []charRule{},
		},
		{
			name: "unsafe reducers of the two phases go to their own lists",
			changelog: r3diff.Changelog{
				charChange(".spec.distribution.modules.monitoring.type", "prometheus", "none"),
				charChange(".spec.infrastructure.nodes.0.hostname", "node-a", "node-b"),
			},
			wantInfrastructure: []charRule{{".spec.infrastructure.nodes.*.hostname", "node-a", "node-b"}},
			wantDistribution:   []charRule{{".spec.distribution.modules.monitoring.type", "prometheus", "none"}},
		},
	}
}

// charBaseUnsafeReducers repeats the extractor calls of creator.go before it took its
// lists from the shared verdict: the infrastructure reducers, then the distribution ones, both
// on the changes as reported.
func charBaseUnsafeReducers(r rules.Extractor, changelog r3diff.Changelog) ([]rules.Rule, []rules.Rule) {
	unsafeReducersInfrastructure := r.UnsafeReducerRulesByDiffs(
		r.GetReducers(
			cluster.OperationPhaseInfrastructure,
		),
		changelog,
	)

	unsafeReducersDistribution := r.UnsafeReducerRulesByDiffs(
		r.GetReducers(
			cluster.OperationPhaseDistribution,
		),
		changelog,
	)

	return unsafeReducersInfrastructure, unsafeReducersDistribution
}

func TestCreator_UnsafeReducersCharacterization(t *testing.T) {
	t.Parallel()

	for _, tC := range charUnsafeReducersCases() {
		t.Run(tC.name, func(t *testing.T) {
			t.Parallel()

			distroPath := characterizedRules
			if tC.noRules {
				distroPath = t.TempDir()
			}

			infrastructure, distribution := charBaseUnsafeReducers(charExtractor(t, distroPath), tC.changelog)

			assert.Equal(t, tC.wantInfrastructure, charRules(infrastructure), "infrastructure")
			assert.Equal(t, tC.wantDistribution, charRules(distribution), "distribution")
		})
	}
}
