// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package create //nolint:testpackage // builds a PreFlight with only its paths, which are unexported.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/immutable/supported"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/distribution"
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

// The two rule checks of the apply preflight of Immutable and the verdict of the diff
// decide on the same changelog and the same rule file. The preflight takes its
// decisions from the verdict, so each side is checked against the expected result,
// not against the other: the errors of the preflight, with their exact text, and the
// changes the verdict blocks, with the text the table shows.
func TestPreFlight_RuleChecksAgreeWithTheVerdict(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		current     string
		updated     string
		wantState   string
		wantReducer string
		// The text of each blocked change, by path.
		wantBlocked map[string]string
	}{
		{
			name:      "an immutable field changed",
			current:   `{spec: {kubernetes: {networking: {podCIDR: 172.16.128.0/17}}}}`,
			updated:   `{spec: {kubernetes: {networking: {podCIDR: 10.42.0.0/16}}}}`,
			wantState: charImmutablePrefix + ".spec.kubernetes.networking.podCIDR  oldValue 172.16.128.0/17 newValue 10.42.0.0/16",
			wantBlocked: map[string]string{
				".spec.kubernetes.networking.podCIDR": "Pod CIDR cannot be changed after cluster creation",
			},
		},
		{
			name:        "an unsupported transition",
			current:     `{spec: {distribution: {modules: {ingress: {nginx: {type: single}}}}}}`,
			updated:     `{spec: {distribution: {modules: {ingress: {nginx: {type: dual}}}}}}`,
			wantReducer: charUnsupportedPrefix + charNginxReason,
			wantBlocked: map[string]string{".spec.distribution.modules.ingress.nginx.type": charNginxReason},
		},
		{
			name:        "a safe immutable transition",
			current:     `{spec: {distribution: {modules: {logging: {type: none, loki: {tsdbStartDate: "2024-01-01"}}}}}}`,
			updated:     `{spec: {distribution: {modules: {logging: {type: loki, loki: {tsdbStartDate: "2025-01-01"}}}}}}`,
			wantBlocked: map[string]string{},
		},
		{
			name:        "a change with no rule",
			current:     `{spec: {distribution: {modules: {monitoring: {prometheus: {retentionSize: 10GB}}}}}}`,
			updated:     `{spec: {distribution: {modules: {monitoring: {prometheus: {retentionSize: 10GB, retentionTime: 15d}}}}}}`,
			wantBlocked: map[string]string{},
		},
		{
			name:        "a whole object added where apply does not expand",
			current:     `{spec: {kubernetes: {advanced: {unchanged: true}}}}`,
			updated:     `{spec: {kubernetes: {advanced: {unchanged: true, kubeProxy: {type: none}}}}}`,
			wantBlocked: map[string]string{},
		},
	}

	for _, tC := range testCases {
		t.Run(tC.name, func(t *testing.T) {
			t.Parallel()

			checker := diffs.NewBaseChecker(agreementConfig(t, tC.current), agreementConfig(t, tC.updated))

			changelog, err := checker.GenerateDiff()
			require.NoError(t, err)
			require.NotEmpty(t, changelog)

			preflight := &PreFlight{paths: cluster.CreatorPaths{DistroPath: agreementRules}}

			assert.Equal(t, tC.wantState, charErrorText(preflight.CheckStateDiffs(changelog, checker)), "immutable check")
			assert.Equal(t, tC.wantReducer, charErrorText(preflight.CheckReducerDiffs(changelog, checker)), "unsupported check")

			extractor, err := rules.NewImmutableClusterRulesExtractor(agreementRules, checker.GetCurrentConfig(), supported.Phases())
			require.NoError(t, err)

			profile, err := diffs.RequireProfileForKind(distribution.ImmutableKind)
			require.NoError(t, err)

			blocked := map[string]string{}

			for _, v := range diffs.Verdicts(changelog, extractor, profile) {
				if v.Verdict == diffs.VerdictBlocked {
					blocked[v.Path()] = v.Text
				}
			}

			assert.Equal(t, tC.wantBlocked, blocked, "blocked changes of the verdict")
		})
	}
}
