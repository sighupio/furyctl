// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package create //nolint:testpackage // builds a PreFlight with only its paths, which are unexported.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/kfddistribution/supported"
	"github.com/sighupio/furyctl/pkg/diffs"
	rules "github.com/sighupio/furyctl/pkg/rulesextractor"
	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

// agreementRules is the folder that holds the real rule files of the distribution
// (pkg/diffs/testdata/rules, see its PROVENANCE).
const agreementRules = "../../../../../../pkg/diffs/testdata"

func agreementConfig(t *testing.T, content string) map[string]any {
	t.Helper()

	config := map[string]any{}

	require.NoError(t, yamlx.UnmarshalV3([]byte(content), &config))

	return config
}

// The verdict and the two rule checks of the apply preflight of KFDDistribution decide on the
// same changelog and the same rule file: a change is blocked exactly when a check fails.
func TestPreFlight_RuleChecksAgreeWithTheVerdict(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		current     string
		updated     string
		wantBlocked int
		wantReason  string
	}{
		{
			name:        "an immutable field changed",
			current:     `{spec: {distribution: {modules: {networking: {type: calico}}}}}`,
			updated:     `{spec: {distribution: {modules: {networking: {type: cilium}}}}}`,
			wantBlocked: 1,
		},
		{
			name:        "an unsupported transition",
			current:     `{spec: {distribution: {modules: {ingress: {nginx: {type: single}}}}}}`,
			updated:     `{spec: {distribution: {modules: {ingress: {nginx: {type: dual}}}}}}`,
			wantBlocked: 1,
		},
		{
			name:        "a safe immutable transition",
			current:     `{spec: {distribution: {modules: {logging: {type: none, loki: {tsdbStartDate: "2024-01-01"}}}}}}`,
			updated:     `{spec: {distribution: {modules: {logging: {type: loki, loki: {tsdbStartDate: "2025-01-01"}}}}}}`,
			wantBlocked: 0,
		},
		{
			name:        "a change with no rule",
			current:     `{spec: {distribution: {modules: {monitoring: {prometheus: {retentionSize: 10GB}}}}}}`,
			updated:     `{spec: {distribution: {modules: {monitoring: {prometheus: {retentionSize: 10GB, retentionTime: 15d}}}}}}`,
			wantBlocked: 0,
		},
		{
			name:        "a whole object added where apply does not expand",
			current:     `{spec: {kubernetes: {advanced: {unchanged: true}}}}`,
			updated:     `{spec: {kubernetes: {advanced: {unchanged: true, kubeProxy: {type: none}}}}}`,
			wantBlocked: 0,
		},
	}

	for _, tC := range testCases {
		t.Run(tC.name, func(t *testing.T) {
			t.Parallel()

			checker := diffs.NewBaseChecker(agreementConfig(t, tC.current), agreementConfig(t, tC.updated))

			changelog, err := checker.GenerateDiff()
			require.NoError(t, err)
			require.NotEmpty(t, changelog)

			preflight := &PreFlight{distroPath: agreementRules}

			stateErr := preflight.CheckStateDiffs(changelog, checker)
			reducerErr := preflight.CheckReducerDiffs(changelog, checker)

			extractor, err := rules.NewDistroClusterRulesExtractor(agreementRules, checker.GetCurrentConfig(), supported.Phases())
			require.NoError(t, err)

			profile, ok := diffs.ProfileForKind("KFDDistribution")
			require.True(t, ok)

			blocked := 0

			for _, v := range diffs.Verdicts(changelog, extractor, profile) {
				if v.Verdict == diffs.VerdictBlocked {
					blocked++
				}
			}

			assert.Equal(t, tC.wantBlocked, blocked, "blocked changes")
			assert.Equal(
				t,
				tC.wantBlocked > 0,
				stateErr != nil || reducerErr != nil,
				"preflight rule checks: %v, %v", stateErr, reducerErr,
			)

			if tC.wantReason != "" {
				require.NoError(t, stateErr)
				require.ErrorIs(t, reducerErr, errUnsupported)
				assert.ErrorContains(t, reducerErr, tC.wantReason)
			}
		})
	}
}
