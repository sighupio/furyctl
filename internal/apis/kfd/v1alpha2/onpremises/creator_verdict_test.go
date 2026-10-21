// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package onpremises //nolint:testpackage // calls the unexported function that gives the unsafe reducer lists.

import (
	"testing"

	r3diff "github.com/r3labs/diff/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The creator of OnPremises takes its unsafe reducer lists from the shared verdict. For
// every characterized changelog the lists are the characterized ones, and the ones
// the extractor calls of the base creator give.
func TestCreator_UnsafeReducersFromTheVerdict(t *testing.T) {
	t.Parallel()

	for _, tC := range charUnsafeReducersCases() {
		t.Run(tC.name, func(t *testing.T) {
			t.Parallel()

			distroPath := characterizedRules
			if tC.noRules {
				distroPath = t.TempDir()
			}

			distribution, kubernetes, err := unsafeReducerRules(charExtractor(t, distroPath), tC.changelog)
			require.NoError(t, err)

			assert.Equal(t, tC.wantDistribution, charRules(distribution), "distribution")
			assert.Equal(t, tC.wantKubernetes, charRules(kubernetes), "kubernetes")

			baseDistribution, baseKubernetes := charBaseUnsafeReducers(charExtractor(t, distroPath), tC.changelog)

			assert.Equal(t, charRules(baseDistribution), charRules(distribution), "distribution of the base creator")
			assert.Equal(t, charRules(baseKubernetes), charRules(kubernetes), "kubernetes of the base creator")
		})
	}
}

// The creator of OnPremises reads the reducer rules of two phases: the distribution ones
// on the changes as reported, the kubernetes ones on the changes expanded into leaves.
// The rule file is written for this test, with one reducer rule per phase on the same
// field and one per phase on two leaves of the same object.
func TestCreator_UnsafeReducersPerPhaseOfTheProfile(t *testing.T) {
	t.Parallel()

	const syntheticRules = "../../../../../pkg/diffs/testdata/synthetic"

	testCases := []struct {
		name             string
		changelog        r3diff.Changelog
		wantDistribution []charRule
		wantKubernetes   []charRule
	}{
		{
			name: "a field with a reducer rule in each phase is in both lists",
			changelog: r3diff.Changelog{
				charChange(".spec.kubernetes.advanced.scheduler.profile", "default", "strict"),
			},
			wantDistribution: []charRule{{".spec.kubernetes.advanced.scheduler.profile", "default", "strict"}},
			wantKubernetes:   []charRule{{".spec.kubernetes.advanced.scheduler.profile", "default", "strict"}},
		},
		{
			name: "an added object reaches the kubernetes rule of its leaf, not the distribution one",
			changelog: r3diff.Changelog{
				{
					Type: r3diff.CREATE,
					Path: []string{"spec", "kubernetes", "advanced", "encryption"},
					From: nil,
					To:   map[string]any{"provider": "aescbc", "mode": "full"},
				},
			},
			wantDistribution: []charRule{},
			wantKubernetes:   []charRule{{".spec.kubernetes.advanced.encryption.provider", nil, "aescbc"}},
		},
	}

	for _, tC := range testCases {
		t.Run(tC.name, func(t *testing.T) {
			t.Parallel()

			distribution, kubernetes, err := unsafeReducerRules(charExtractor(t, syntheticRules), tC.changelog)
			require.NoError(t, err)

			assert.Equal(t, tC.wantDistribution, charRules(distribution), "distribution")
			assert.Equal(t, tC.wantKubernetes, charRules(kubernetes), "kubernetes")
		})
	}
}
