// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package create //nolint:testpackage // builds a PreFlight with only its paths, which are unexported.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/ekscluster/supported"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/pkg/diffs"
	rules "github.com/sighupio/furyctl/pkg/rulesextractor"
	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

const (
	// agreementRules is the folder that holds the real rule files of the distribution
	// (pkg/diffs/testdata/rules, see its PROVENANCE).
	agreementRules = "../../../../../../pkg/diffs/testdata"

	// syntheticRules is the folder that holds rule files written for the tests of the
	// profiles: the EKSCluster one has immutable and unsupported rules in the
	// infrastructure and kubernetes phases, which the real rules of the kind leave empty.
	syntheticRules = "../../../../../../pkg/diffs/testdata/synthetic"
)

func agreementConfig(t *testing.T, content string) map[string]any {
	t.Helper()

	config := map[string]any{}

	require.NoError(t, yamlx.UnmarshalV3([]byte(content), &config))

	return config
}

// The verdict and the two rule checks of the apply preflight of EKSCluster decide on the
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
			current:     `{spec: {distribution: {modules: {logging: {type: loki, loki: {tsdbStartDate: "2024-01-01"}}}}}}`,
			updated:     `{spec: {distribution: {modules: {logging: {type: loki, loki: {tsdbStartDate: "2025-01-01"}}}}}}`,
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

			preflight := &PreFlight{paths: cluster.CreatorPaths{DistroPath: agreementRules}}

			stateErr := preflight.CheckImmutablesDiffs(changelog, checker)
			reducerErr := preflight.CheckReducersDiffs(changelog, checker)

			extractor, err := rules.NewEKSClusterRulesExtractor(agreementRules, checker.GetCurrentConfig(), supported.Phases())
			require.NoError(t, err)

			profile, ok := diffs.ProfileForKind("EKSCluster")
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

// The preflight of EKSCluster takes its rule decisions by itself, so it is an oracle
// for the infrastructure and kubernetes phases of the profile of the kind.
func TestPreFlight_InfrastructureAndKubernetesChecksAgreeWithTheVerdict(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		current     string
		updated     string
		wantState   string
		wantReducer string
		wantBlocked map[string]string
	}{
		{
			name:      "an immutable infrastructure field changed",
			current:   `{spec: {infrastructure: {vpc: {network: {cidr: 10.0.0.0/16}}}}}`,
			updated:   `{spec: {infrastructure: {vpc: {network: {cidr: 10.1.0.0/16}}}}}`,
			wantState: "immutable path changed: immutable value changed: path .spec.infrastructure.vpc.network.cidr  oldValue 10.0.0.0/16 newValue 10.1.0.0/16",
			wantBlocked: map[string]string{
				".spec.infrastructure.vpc.network.cidr": "the CIDR of the VPC cannot be changed",
			},
		},
		{
			name:        "an unsupported infrastructure transition",
			current:     `{spec: {infrastructure: {vpn: {instances: 2}}}}`,
			updated:     `{spec: {infrastructure: {vpn: {instances: 1}}}}`,
			wantReducer: "unsupported reducer values detected: unsupported value changed: removing a VPN instance is not supported",
			wantBlocked: map[string]string{
				".spec.infrastructure.vpn.instances": "removing a VPN instance is not supported",
			},
		},
		{
			name:      "an immutable kubernetes field changed",
			current:   `{spec: {kubernetes: {serviceIpV4Cidr: 172.20.0.0/16}}}`,
			updated:   `{spec: {kubernetes: {serviceIpV4Cidr: 172.21.0.0/16}}}`,
			wantState: "immutable path changed: immutable value changed: path .spec.kubernetes.serviceIpV4Cidr  oldValue 172.20.0.0/16 newValue 172.21.0.0/16",
			wantBlocked: map[string]string{
				".spec.kubernetes.serviceIpV4Cidr": "the service CIDR cannot be changed",
			},
		},
		{
			name:        "an unsupported kubernetes transition",
			current:     `{spec: {kubernetes: {nodePoolsLaunchKind: launch_templates}}}`,
			updated:     `{spec: {kubernetes: {nodePoolsLaunchKind: launch_configurations}}}`,
			wantReducer: "unsupported reducer values detected: unsupported value changed: going back to launch configurations is not supported",
			wantBlocked: map[string]string{
				".spec.kubernetes.nodePoolsLaunchKind": "going back to launch configurations is not supported",
			},
		},
		{
			name:        "a supported kubernetes transition",
			current:     `{spec: {kubernetes: {nodePoolsLaunchKind: launch_configurations}}}`,
			updated:     `{spec: {kubernetes: {nodePoolsLaunchKind: launch_templates}}}`,
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

			preflight := &PreFlight{paths: cluster.CreatorPaths{DistroPath: syntheticRules}}

			assert.Equal(t, tC.wantState, errorText(preflight.CheckImmutablesDiffs(changelog, checker)), "immutable check")
			assert.Equal(t, tC.wantReducer, errorText(preflight.CheckReducersDiffs(changelog, checker)), "unsupported check")

			extractor, err := rules.NewEKSClusterRulesExtractor(syntheticRules, checker.GetCurrentConfig(), supported.Phases())
			require.NoError(t, err)

			profile, ok := diffs.ProfileForKind("EKSCluster")
			require.True(t, ok)

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

func errorText(err error) string {
	if err == nil {
		return ""
	}

	return err.Error()
}
