// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package create //nolint:testpackage // builds a PreFlight with only its paths, which are unexported.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	r3diff "github.com/r3labs/diff/v3"
	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/pkg/diffs"
)

// These tests pin what the two rule checks of the apply preflight of Immutable return
// against the real rule files of the distribution: error or no error, and the exact
// text. They were written on the apply code before it took its decisions from the
// shared verdict, and are not edited afterwards.

// characterizedRules is the folder that holds the real rule files of the distribution
// (pkg/diffs/testdata/rules, see its PROVENANCE).
const characterizedRules = "../../../../../../pkg/diffs/testdata"

const (
	charImmutablePrefix   = "immutable path changed: immutable value changed: path "
	charUnsupportedPrefix = "unsupported reducer values detected: unsupported value changed: "
	charNginxReason       = "currently, switching Ingress from single to dual nginx is not supported. " +
		"You need to first remove the current stack with type: none."
	charMimirReason = "switching from Mimir to Prometheus Agent is not currently supported. " +
		"You need to first remove the current stack with type: none."
)

// charChange is one change of a changelog, written with its dotted path.
func charChange(path string, from, to any) r3diff.Change {
	return r3diff.Change{
		Type: r3diff.UPDATE,
		Path: strings.Split(strings.TrimPrefix(path, "."), "."),
		From: from,
		To:   to,
	}
}

// charConfig is a current configuration that holds only the logging type, the one
// value the rules read from it when it did not change.
func charConfig(loggingType string) map[string]any {
	return map[string]any{
		"spec": map[string]any{
			"distribution": map[string]any{
				"modules": map[string]any{
					"logging": map[string]any{"type": loggingType},
				},
			},
		},
	}
}

func charErrorText(err error) string {
	if err == nil {
		return ""
	}

	return err.Error()
}

func TestPreFlight_RuleChecksCharacterization(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		rules       string
		current     map[string]any
		changelog   r3diff.Changelog
		wantState   string
		wantReducer string
	}{
		{
			name:    "a change with no rule passes the two checks",
			current: charConfig("loki"),
			changelog: r3diff.Changelog{
				charChange(".spec.distribution.modules.monitoring.prometheus.retentionTime", nil, "15d"),
			},
		},
		{
			name:    "an immutable field fails the immutable check",
			current: charConfig("loki"),
			changelog: r3diff.Changelog{
				charChange(".spec.kubernetes.networking.podCIDR", "172.16.128.0/17", "10.42.0.0/16"),
			},
			wantState: charImmutablePrefix + ".spec.kubernetes.networking.podCIDR  oldValue 172.16.128.0/17 newValue 10.42.0.0/16",
		},
		{
			name:    "the immutable violations follow the order of the phases, not of the changelog",
			current: charConfig("loki"),
			changelog: r3diff.Changelog{
				charChange(".spec.distribution.modules.networking.type", "calico", "cilium"),
				charChange(".spec.kubernetes.networking.podCIDR", "172.16.128.0/17", "10.42.0.0/16"),
			},
			wantState: charImmutablePrefix + ".spec.kubernetes.networking.podCIDR  oldValue 172.16.128.0/17 newValue 10.42.0.0/16" +
				"\nimmutable value changed: path " + ".spec.distribution.modules.networking.type  oldValue calico newValue cilium",
		},
		{
			name:    "a whole object added at an immutable path is one immutable violation with the object as its value",
			current: charConfig("loki"),
			changelog: r3diff.Changelog{
				charChange(".spec.kubernetes.etcd", nil, map[string]any{"dataDir": "/var/lib/etcd", "quota": "8G"}),
			},
			wantState: charImmutablePrefix + ".spec.kubernetes.etcd  oldValue <nil> newValue map[dataDir:/var/lib/etcd quota:8G]",
		},
		{
			name:    "a whole list item removed is one immutable violation",
			current: charConfig("loki"),
			changelog: r3diff.Changelog{
				charChange(".spec.kubernetes.controlPlane.members.2", map[string]any{"hostname": "cp3"}, nil),
			},
			wantState: charImmutablePrefix + ".spec.kubernetes.controlPlane.members.2  oldValue map[hostname:cp3] newValue <nil>",
		},
		{
			name:    "an immutable leaf changed with its object on both sides fails the immutable check",
			current: charConfig("loki"),
			changelog: r3diff.Changelog{
				charChange(".spec.kubernetes.advanced.kubeProxy.type", "ipvs", "nftables"),
			},
			wantState: charImmutablePrefix + ".spec.kubernetes.advanced.kubeProxy.type  oldValue ipvs newValue nftables",
		},
		{
			name:    "an immutable leaf inside a whole object added passes the two checks",
			current: charConfig("loki"),
			changelog: r3diff.Changelog{
				charChange(".spec.kubernetes.advanced.kubeProxy", nil, map[string]any{"type": "nftables"}),
			},
		},
		{
			name:    "an unsupported transition of a rule with several of them gives the matching reason",
			current: charConfig("loki"),
			changelog: r3diff.Changelog{
				charChange(".spec.distribution.modules.monitoring.type", "mimir", "prometheusAgent"),
			},
			wantReducer: charUnsupportedPrefix + charMimirReason,
		},
		{
			name:    "the unsupported violations follow the order of the changelog inside a phase",
			current: charConfig("loki"),
			changelog: r3diff.Changelog{
				charChange(".spec.distribution.modules.ingress.nginx.type", "single", "dual"),
				charChange(".spec.distribution.modules.monitoring.type", "mimir", "prometheusAgent"),
			},
			wantReducer: charUnsupportedPrefix + charNginxReason +
				"\nunsupported value changed: " + charMimirReason,
		},
		{
			name:    "an unsupported transition fails the unsupported check with its reason",
			current: charConfig("loki"),
			changelog: r3diff.Changelog{
				charChange(".spec.distribution.modules.ingress.nginx.type", "single", "dual"),
			},
			wantReducer: charUnsupportedPrefix + charNginxReason,
		},
		{
			name:    "an immutable field whose safe conditions are not met fails the immutable check",
			current: charConfig("loki"),
			changelog: r3diff.Changelog{
				charChange(".spec.distribution.modules.logging.loki.tsdbStartDate", "2024-01-01", "2025-01-01"),
			},
			wantState: charImmutablePrefix + ".spec.distribution.modules.logging.loki.tsdbStartDate  oldValue 2024-01-01 newValue 2025-01-01",
		},
		{
			name:    "an immutable field whose safe condition is met by another change passes",
			current: charConfig("loki"),
			changelog: r3diff.Changelog{
				charChange(".spec.distribution.modules.logging.loki.tsdbStartDate", "2024-01-01", "2025-01-01"),
				charChange(".spec.distribution.modules.logging.type", "none", "loki"),
			},
		},
		{
			name:    "an immutable field whose safe condition is met by the current configuration passes",
			current: charConfig("none"),
			changelog: r3diff.Changelog{
				charChange(".spec.distribution.modules.logging.loki.tsdbStartDate", "2024-01-01", "2025-01-01"),
			},
		},
		{
			name:    "an immutable change and an unsupported transition fail one check each",
			current: charConfig("loki"),
			changelog: r3diff.Changelog{
				charChange(".spec.distribution.modules.ingress.nginx.type", "single", "dual"),
				charChange(".spec.kubernetes.networking.podCIDR", "172.16.128.0/17", "10.42.0.0/16"),
			},
			wantState:   charImmutablePrefix + ".spec.kubernetes.networking.podCIDR  oldValue 172.16.128.0/17 newValue 10.42.0.0/16",
			wantReducer: charUnsupportedPrefix + charNginxReason,
		},
		{
			name: "an unsupported transition without reason gets the generic text with the rule path",
			rules: "kubernetes:\n  - path: .spec.kubernetes.nodes.*.role\n    immutable: false\n" +
				"    unsupported:\n      - from: worker\n        to: infra\n",
			current: charConfig("loki"),
			changelog: r3diff.Changelog{
				charChange(".spec.kubernetes.nodes.1.role", "worker", "infra"),
			},
			wantReducer: charUnsupportedPrefix + "changing .spec.kubernetes.nodes.*.role from worker to infra is not supported",
		},
	}

	for _, tC := range testCases {
		t.Run(tC.name, func(t *testing.T) {
			t.Parallel()

			distroPath := characterizedRules

			if tC.rules != "" {
				distroPath = t.TempDir()

				require.NoError(t, os.Mkdir(filepath.Join(distroPath, "rules"), 0o750))
				require.NoError(t, os.WriteFile(
					filepath.Join(distroPath, "rules", "immutable-kfd-v1alpha2.yaml"),
					[]byte(tC.rules),
					0o600,
				))
			}

			checker := diffs.NewBaseChecker(tC.current, map[string]any{})
			preflight := &PreFlight{paths: cluster.CreatorPaths{DistroPath: distroPath}}

			assert.Equal(t, tC.wantState, charErrorText(preflight.CheckStateDiffs(tC.changelog, checker)))
			assert.Equal(t, tC.wantReducer, charErrorText(preflight.CheckReducerDiffs(tC.changelog, checker)))
		})
	}
}

func TestPreFlight_RuleChecksCharacterization_Sentinels(t *testing.T) {
	t.Parallel()

	t.Run("the errors of the two checks wrap the sentinels of the preflight", func(t *testing.T) {
		t.Parallel()

		checker := diffs.NewBaseChecker(charConfig("loki"), map[string]any{})
		preflight := &PreFlight{paths: cluster.CreatorPaths{DistroPath: characterizedRules}}
		changelog := r3diff.Changelog{
			charChange(".spec.kubernetes.networking.podCIDR", "172.16.128.0/17", "10.42.0.0/16"),
			charChange(".spec.distribution.modules.ingress.nginx.type", "single", "dual"),
		}

		require.ErrorIs(t, preflight.CheckStateDiffs(changelog, checker), errImmutable)
		require.ErrorIs(t, preflight.CheckReducerDiffs(changelog, checker), errUnsupported)
	})

	t.Run("without a rule file the two checks pass", func(t *testing.T) {
		t.Parallel()

		checker := diffs.NewBaseChecker(charConfig("loki"), map[string]any{})
		preflight := &PreFlight{paths: cluster.CreatorPaths{DistroPath: t.TempDir()}}
		changelog := r3diff.Changelog{
			charChange(".spec.kubernetes.networking.podCIDR", "172.16.128.0/17", "10.42.0.0/16"),
			charChange(".spec.distribution.modules.ingress.nginx.type", "single", "dual"),
		}

		require.NoError(t, preflight.CheckStateDiffs(changelog, checker))
		require.NoError(t, preflight.CheckReducerDiffs(changelog, checker))
	})
}

// The rules extractor logs at error level each fromNodes condition whose path is
// neither in the changes nor in the current configuration. The test is not parallel
// because it reads the global logger.
//
//nolint:paralleltest // the hook is on the global logger of logrus.
func TestPreFlight_RuleChecksCharacterization_Logs(t *testing.T) {
	t.Run("the error log lines of each check stay the same", func(t *testing.T) {
		hook := logtest.NewGlobal()
		t.Cleanup(hook.Reset)

		// The current configuration has no logging type and the changelog does not change it.
		checker := diffs.NewBaseChecker(map[string]any{}, map[string]any{})
		preflight := &PreFlight{paths: cluster.CreatorPaths{DistroPath: characterizedRules}}
		changelog := r3diff.Changelog{
			charChange(".spec.distribution.modules.monitoring.prometheus.retentionTime", nil, "15d"),
		}

		errorLines := func() []string {
			lines := []string{}

			for _, entry := range hook.AllEntries() {
				if entry.Level == logrus.ErrorLevel {
					lines = append(lines, entry.Message)
				}
			}

			return lines
		}

		const missingLoggingType = "error getting value for .spec.distribution.modules.logging.type: " +
			"key 'spec' not found in path"

		require.NoError(t, preflight.CheckStateDiffs(changelog, checker))
		assert.Equal(
			t,
			[]string{missingLoggingType, missingLoggingType, missingLoggingType, missingLoggingType},
			errorLines(),
		)

		hook.Reset()

		require.NoError(t, preflight.CheckReducerDiffs(changelog, checker))
		assert.Empty(t, errorLines())
	})
}
