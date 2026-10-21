// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package diffs_test

import (
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	r3diff "github.com/r3labs/diff/v3"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	eks "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/ekscluster/supported"
	immutable "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/immutable/supported"
	distro "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/kfddistribution/supported"
	onprem "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/onpremises/supported"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/pkg/diffs"
	rules "github.com/sighupio/furyctl/pkg/rulesextractor"
)

const (
	// The folder that holds rules/<kind>-kfd-v1alpha2.yaml, the real rule files of
	// the distribution (see testdata/rules/PROVENANCE).
	realRules = "testdata"

	kindOnPremises      = "OnPremises"
	kindImmutable       = "Immutable"
	kindEKSCluster      = "EKSCluster"
	kindKFDDistribution = "KFDDistribution"

	phaseInfrastructure = "infrastructure"
	phaseKubernetes     = "kubernetes"
	phaseDistribution   = "distribution"

	pathLoggingType   = ".spec.distribution.modules.logging.type"
	pathNginxType     = ".spec.distribution.modules.ingress.nginx.type"
	pathKubeProxy     = ".spec.kubernetes.advanced.kubeProxy"
	pathKubeProxyType = ".spec.kubernetes.advanced.kubeProxy.type"
	pathPodCIDR       = ".spec.kubernetes.networking.podCIDR"
	pathRetentionTime = ".spec.distribution.modules.monitoring.prometheus.retentionTime"

	// The reason of the unsupported transition of the real Immutable rules for the
	// nginx type from single to dual.
	reasonNginxSingleToDual = "currently, switching Ingress from single to dual nginx is not supported. " +
		"You need to first remove the current stack with type: none."
)

// field is one path of the two configurations. A nil side means the field is absent there.
type field struct {
	path string
	from any
	to   any
}

// configs builds the current and the new configuration. Every parent object of a
// field exists on both sides, so the changelog reports the field itself and not
// one of its parents.
func configs(fields ...field) (map[string]any, map[string]any) {
	current := map[string]any{}
	updated := map[string]any{}

	for _, f := range fields {
		parent := f.path[:strings.LastIndex(f.path, ".")]

		for _, side := range []struct {
			config map[string]any
			value  any
		}{{current, f.from}, {updated, f.to}} {
			setPath(side.config, parent+".unchanged", true)

			if side.value != nil {
				setPath(side.config, f.path, side.value)
			}
		}
	}

	return current, updated
}

func setPath(config map[string]any, path string, value any) {
	keys := strings.Split(strings.TrimPrefix(path, "."), ".")

	for _, key := range keys[:len(keys)-1] {
		child, ok := config[key].(map[string]any)
		if !ok {
			child = map[string]any{}
			config[key] = child
		}

		config = child
	}

	config[keys[len(keys)-1]] = value
}

// kubeProxyAdded is the whole object .spec.kubernetes.advanced.kubeProxy added with
// the given type, which the changelog reports as one change of the object.
func kubeProxyAdded(proxyType string) field {
	return field{pathKubeProxy, nil, map[string]any{"type": proxyType}}
}

// etcdAdded is the whole object .spec.kubernetes.etcd added with two leaves. The real
// Immutable rules declare the object itself immutable.
func etcdAdded() field {
	return field{".spec.kubernetes.etcd", nil, map[string]any{"dataDir": "/var/lib/etcd", "quota": "8G"}}
}

// nodeRenamed is a change of the hostname of the first infrastructure node.
func nodeRenamed() field {
	return field{
		".spec.infrastructure.nodes",
		[]any{map[string]any{"hostname": "node-a"}},
		[]any{map[string]any{"hostname": "node-b"}},
	}
}

// extractorFor builds the extractor of a kind as its apply does.
func extractorFor(t *testing.T, kind, distroPath string, config map[string]any) rules.Extractor {
	t.Helper()

	var (
		extractor rules.Extractor
		err       error
	)

	switch kind {
	case kindOnPremises:
		extractor, err = rules.NewOnPremClusterRulesExtractor(distroPath, config, onprem.Phases())

	case kindImmutable:
		extractor, err = rules.NewImmutableClusterRulesExtractor(distroPath, config, immutable.Phases())

	case kindEKSCluster:
		extractor, err = rules.NewEKSClusterRulesExtractor(distroPath, config, eks.Phases())

	case kindKFDDistribution:
		extractor, err = rules.NewDistroClusterRulesExtractor(distroPath, config, distro.Phases())

	default:
		t.Fatalf("unknown kind %q", kind)
	}

	require.NoError(t, err)

	return extractor
}

// verdictsFor computes the verdicts as the diff does: the changelog between the two
// configurations, the rules of the kind and the current configuration for the rules.
func verdictsFor(t *testing.T, kind, distroPath string, fields ...field) []diffs.ChangeVerdict {
	t.Helper()

	current, updated := configs(fields...)

	changelog, err := diffs.NewBaseChecker(current, updated).GenerateDiff()
	require.NoError(t, err)

	profile, ok := diffs.ProfileForKind(kind)
	require.True(t, ok, "no profile for kind %s", kind)

	return diffs.Verdicts(changelog, extractorFor(t, kind, distroPath, current), profile)
}

func verdictAt(t *testing.T, verdicts []diffs.ChangeVerdict, path string) diffs.ChangeVerdict {
	t.Helper()

	for _, v := range verdicts {
		if v.Path() == path {
			return v
		}
	}

	require.FailNowf(t, "no leaf change", "no leaf change at %s in %d verdicts", path, len(verdicts))

	return diffs.ChangeVerdict{}
}

// syntheticRules writes a rule file for the Immutable kind with the given content.
func syntheticRules(t *testing.T, content string) string {
	t.Helper()

	distroPath := t.TempDir()

	require.NoError(t, os.Mkdir(filepath.Join(distroPath, "rules"), 0o750))
	require.NoError(t, os.WriteFile(
		filepath.Join(distroPath, "rules", "immutable-kfd-v1alpha2.yaml"),
		[]byte(content),
		0o600,
	))

	return distroPath
}

func TestVerdicts_RealImmutableRules(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name            string
		change          field
		wantVerdict     diffs.Verdict
		wantText        string
		wantImmutable   bool
		wantUnsupported bool
		wantUnsafe      int
	}{
		{
			name:          "an immutable field is blocked with the rule description",
			change:        field{pathPodCIDR, "172.16.128.0/17", "10.42.0.0/16"},
			wantVerdict:   diffs.VerdictBlocked,
			wantText:      "Pod CIDR cannot be changed after cluster creation",
			wantImmutable: true,
		},
		{
			name:            "an unsupported transition is blocked with its reason",
			change:          field{pathNginxType, "single", "dual"},
			wantVerdict:     diffs.VerdictBlocked,
			wantText:        reasonNginxSingleToDual,
			wantUnsupported: true,
			wantUnsafe:      1,
		},
		{
			name:        "an unsafe reducer needs confirmation",
			change:      field{".spec.distribution.common.networkPoliciesEnabled", false, true},
			wantVerdict: diffs.VerdictNeedsConfirmation,
			wantText: "changes to the network policies have been detected. " +
				"This will cause the reconfiguration or deletion of the current network policies.",
			wantUnsafe: 1,
		},
		{
			name:        "a safe reducer is a migration",
			change:      field{pathLoggingType, "none", "loki"},
			wantVerdict: diffs.VerdictMigration,
			wantText: "changes to the logging module type have been detected. " +
				"This will cause the reconfiguration or deletion of the current logging stack.",
		},
		{
			name:        "a field without a rule has no verdict",
			change:      field{pathRetentionTime, nil, "15d"},
			wantVerdict: diffs.VerdictNone,
			wantText:    "",
		},
		{
			name:            "blocked wins over needs confirmation",
			change:          field{pathNginxType, "single", "dual"},
			wantVerdict:     diffs.VerdictBlocked,
			wantText:        reasonNginxSingleToDual,
			wantUnsupported: true,
			wantUnsafe:      1,
		},
	}

	for _, tC := range testCases {
		t.Run(tC.name, func(t *testing.T) {
			t.Parallel()

			verdicts := verdictsFor(t, kindImmutable, realRules, tC.change)

			require.Len(t, verdicts, 1)

			got := verdictAt(t, verdicts, tC.change.path)

			assert.Equal(t, tC.wantVerdict, got.Verdict)
			assert.Equal(t, tC.wantText, got.Text)
			assert.Equal(t, tC.wantImmutable, got.Immutable, "immutable violation")
			assert.Equal(t, tC.wantUnsupported, got.Unsupported, "unsupported violation")
			assert.Equal(t, tC.change.from, got.Change.From)
			assert.Equal(t, tC.change.to, got.Change.To)
			assert.Empty(t, got.UnsafeReducers[phaseInfrastructure])
			require.Len(t, got.UnsafeReducers[phaseDistribution], tC.wantUnsafe)

			if tC.wantUnsafe > 0 {
				assert.Equal(t, tC.change.path, got.UnsafeReducers[phaseDistribution][0].Path)
			}
		})
	}
}

func TestVerdicts_SyntheticRules(t *testing.T) {
	t.Parallel()

	t.Run("an immutable rule whose safe condition is met does not block", func(t *testing.T) {
		t.Parallel()

		distroPath := syntheticRules(t, `distribution:
  - path: .spec.distribution.modules.logging.type
    immutable: true
    description: "the logging type cannot be changed"
    safe:
      - from: none
        to: loki
`)

		safe := verdictsFor(t, kindImmutable, distroPath, field{pathLoggingType, "none", "loki"})

		got := verdictAt(t, safe, pathLoggingType)
		assert.Equal(t, diffs.VerdictNone, got.Verdict)
		assert.False(t, got.Immutable)
		assert.Empty(t, got.Text)

		// The same rule blocks a transition its safe condition does not name.
		unsafe := verdictsFor(t, kindImmutable, distroPath, field{pathLoggingType, "loki", "none"})

		got = verdictAt(t, unsafe, pathLoggingType)
		assert.Equal(t, diffs.VerdictBlocked, got.Verdict)
		assert.True(t, got.Immutable)
	})

	t.Run("a rule without description or reason gives an empty text", func(t *testing.T) {
		t.Parallel()

		distroPath := syntheticRules(t, `kubernetes:
  - path: .spec.kubernetes.networking.podCIDR
    immutable: true
`)

		verdicts := verdictsFor(t, kindImmutable, distroPath, field{pathPodCIDR, "172.16.128.0/17", "10.42.0.0/16"})

		got := verdictAt(t, verdicts, pathPodCIDR)
		assert.Equal(t, diffs.VerdictBlocked, got.Verdict)
		assert.True(t, got.Immutable)
		assert.Empty(t, got.Text)
	})

	t.Run("an unsupported transition without reason gives the rule description", func(t *testing.T) {
		t.Parallel()

		distroPath := syntheticRules(t, `distribution:
  - path: .spec.distribution.modules.logging.type
    immutable: false
    description: "the logging type changed"
    unsupported:
      - from: loki
        to: opensearch
`)

		verdicts := verdictsFor(t, kindImmutable, distroPath, field{pathLoggingType, "loki", "opensearch"})

		got := verdictAt(t, verdicts, pathLoggingType)
		assert.Equal(t, diffs.VerdictBlocked, got.Verdict)
		assert.True(t, got.Unsupported)
		assert.Equal(t, "the logging type changed", got.Text)
	})
}

func TestVerdicts_KindProfile(t *testing.T) {
	t.Parallel()

	t.Run("a leaf inside an added object reaches a leaf rule where apply expands", func(t *testing.T) {
		t.Parallel()

		verdicts := verdictsFor(t, kindOnPremises, realRules, kubeProxyAdded("none"))

		require.Len(t, verdicts, 1)

		got := verdictAt(t, verdicts, pathKubeProxyType)

		assert.Equal(t, diffs.VerdictBlocked, got.Verdict)
		assert.Equal(
			t,
			"disabling kube-proxy on an existing cluster is not supported and requires manual intervention "+
				"to remove the kube-proxy DaemonSet.",
			got.Text,
		)
		assert.True(t, got.Unsupported)
		assert.False(t, got.Immutable)
		assert.Nil(t, got.Change.From)
		assert.Equal(t, "none", got.Change.To)
		assert.Equal(t, []string{"spec", "kubernetes", "advanced", "kubeProxy"}, got.Reported.Path)
		assert.Empty(t, got.UnsafeReducers[phaseDistribution])
		require.Len(t, got.UnsafeReducers[phaseKubernetes], 1)
		assert.Equal(t, pathKubeProxyType, got.UnsafeReducers[phaseKubernetes][0].Path)
	})

	t.Run("the unsafe reducer of an added leaf needs confirmation where apply expands", func(t *testing.T) {
		t.Parallel()

		verdicts := verdictsFor(t, kindOnPremises, realRules, kubeProxyAdded("nftables"))

		got := verdictAt(t, verdicts, pathKubeProxyType)

		assert.Equal(t, diffs.VerdictNeedsConfirmation, got.Verdict)
		assert.Equal(t, "changes to kubeProxy.type have been detected.", got.Text)
		assert.False(t, got.Unsupported)
		require.Len(t, got.UnsafeReducers[phaseKubernetes], 1)
	})

	t.Run("a leaf inside an added object does not reach a leaf rule where apply does not expand",
		func(t *testing.T) {
			t.Parallel()

			added := verdictsFor(t, kindImmutable, realRules, kubeProxyAdded("nftables"))

			require.Len(t, added, 1)

			got := verdictAt(t, added, pathKubeProxyType)
			assert.Equal(t, diffs.VerdictNone, got.Verdict)
			assert.False(t, got.Immutable)
			assert.Empty(t, got.Text)

			changed := verdictsFor(t, kindImmutable, realRules, field{pathKubeProxyType, "ipvs", "nftables"})

			got = verdictAt(t, changed, pathKubeProxyType)
			assert.Equal(t, diffs.VerdictBlocked, got.Verdict)
			assert.True(t, got.Immutable)
			assert.Equal(t, "Changes to kube-proxy backend are not currently supported", got.Text)
		})

	t.Run("a check that does not expand reaches every leaf of the object its rule names",
		func(t *testing.T) {
			t.Parallel()

			// The Immutable rules declare the object .spec.kubernetes.etcd immutable, so
			// the whole-object change itself matches and both its leaves are blocked.
			verdicts := verdictsFor(t, kindImmutable, realRules, etcdAdded())

			require.Len(t, verdicts, 2)

			for _, leaf := range []string{".spec.kubernetes.etcd.dataDir", ".spec.kubernetes.etcd.quota"} {
				got := verdictAt(t, verdicts, leaf)

				assert.Equal(t, diffs.VerdictBlocked, got.Verdict, leaf)
				assert.True(t, got.Immutable, leaf)
				assert.Equal(t, "etcd configuration cannot be changed after cluster creation", got.Text, leaf)
				assert.Equal(t, []string{"spec", "kubernetes", "etcd"}, got.Reported.Path, leaf)
			}
		})

	t.Run("the profiles are the phases and the expansion of each apply", func(t *testing.T) {
		t.Parallel()

		want := map[string]diffs.KindProfile{
			kindOnPremises: {
				CheckPhases:           []string{phaseKubernetes, phaseDistribution},
				UnsafeReducerPhases:   []string{phaseDistribution, phaseKubernetes},
				ExpandedReducerPhases: []string{phaseKubernetes},
			},
			kindImmutable: {
				CheckPhases:           []string{phaseInfrastructure, phaseKubernetes, phaseDistribution},
				UnsafeReducerPhases:   []string{phaseInfrastructure, phaseDistribution},
				ExpandedReducerPhases: nil,
			},
			kindEKSCluster: {
				CheckPhases:           []string{phaseInfrastructure, phaseKubernetes, phaseDistribution},
				UnsafeReducerPhases:   []string{phaseDistribution},
				ExpandedReducerPhases: nil,
			},
			kindKFDDistribution: {
				CheckPhases:           []string{phaseDistribution},
				UnsafeReducerPhases:   []string{phaseDistribution},
				ExpandedReducerPhases: nil,
			},
		}

		for kind, wantProfile := range want {
			got, ok := diffs.ProfileForKind(kind)

			require.True(t, ok, kind)
			assert.Equal(t, wantProfile, got, kind)
		}

		_, ok := diffs.ProfileForKind("Unknown")
		assert.False(t, ok)

		// The profiles name the kinds with their own literals, so that pkg/diffs does
		// not depend on internal/distribution; this keeps them on its list of kinds.
		for _, kind := range distribution.ConfigKinds() {
			_, ok := diffs.ProfileForKind(kind)
			assert.True(t, ok, "no profile for the kind %s of distribution.ConfigKinds", kind)
		}
	})
}

// slot is one field of the configurations with the values it can take, nil for
// absent. Random changelogs pick a subset of slots and one transition each, so no
// two changes overlap and every shape the real rules judge (scalar, whole object,
// list item) appears.
type slot struct {
	path   string
	values []any
}

func propertySlots() []slot {
	return []slot{
		{pathPodCIDR, []any{"172.16.128.0/17", "10.42.0.0/16"}},
		{pathNginxType, []any{"single", "dual", "none"}},
		{".spec.distribution.common.networkPoliciesEnabled", []any{false, true}},
		{pathLoggingType, []any{nil, "none", "loki", "opensearch"}},
		{".spec.distribution.modules.monitoring.type", []any{"none", "prometheus", "mimir"}},
		{pathRetentionTime, []any{nil, "15d", "30d"}},
		{pathKubeProxy, []any{nil, map[string]any{"type": "none"}, map[string]any{"type": "nftables"}}},
		{".spec.kubernetes.etcd", []any{nil, map[string]any{"dataDir": "/var/lib/etcd", "quota": "8G"}}},
		{
			".spec.infrastructure.nodes",
			[]any{[]any{map[string]any{"hostname": "node-a"}}, []any{map[string]any{"hostname": "node-b"}}},
		},
	}
}

// randomFields picks one to four slots and a transition between two distinct values
// of each.
func randomFields(rnd *rand.Rand) []field {
	slots := propertySlots()
	picked := rnd.Perm(len(slots))[:1+rnd.IntN(4)]
	fields := make([]field, 0, len(picked))

	for _, i := range picked {
		n := len(slots[i].values)
		from := rnd.IntN(n)
		to := (from + 1 + rnd.IntN(n-1)) % n

		fields = append(fields, field{slots[i].path, slots[i].values[from], slots[i].values[to]})
	}

	return fields
}

// verdictFacts is what a verdict tells the diff and apply, without the changes it
// carries: two runs agree when these are equal.
type verdictFacts struct {
	path        string
	verdict     diffs.Verdict
	text        string
	immutable   bool
	unsupported bool
	unsafe      map[string][]string
}

func factsOf(verdicts []diffs.ChangeVerdict) []verdictFacts {
	facts := make([]verdictFacts, 0, len(verdicts))

	for _, v := range verdicts {
		unsafe := map[string][]string{}

		for phase, matched := range v.UnsafeReducers {
			unsafe[phase] = rules.Paths(matched)
		}

		facts = append(facts, verdictFacts{v.Path(), v.Verdict, v.Text, v.Immutable, v.Unsupported, unsafe})
	}

	return facts
}

func hasUnsafeReducers(v diffs.ChangeVerdict) bool {
	for _, matched := range v.UnsafeReducers {
		if len(matched) > 0 {
			return true
		}
	}

	return false
}

func leafPaths(changelog r3diff.Changelog) []string {
	paths := make([]string, 0, len(changelog))

	for _, c := range changelog {
		paths = append(paths, "."+strings.Join(c.Path, "."))
	}

	slices.Sort(paths)

	return paths
}

// The invariants of the verdict hold on random changelogs against the real rule
// files of the four kinds: one verdict per leaf change attributed to its reported
// change, the verdict and the facts it exposes agree with the precedence of the
// spec, and the result does not depend on the state the extractor keeps from an
// earlier run.
func TestVerdicts_Properties(t *testing.T) {
	t.Parallel()

	const runs = 100

	for _, kind := range []string{kindOnPremises, kindImmutable, kindEKSCluster, kindKFDDistribution} {
		t.Run("the verdict invariants hold in "+kind, func(t *testing.T) {
			t.Parallel()

			rnd := rand.New(rand.NewPCG(7, 13))

			profile, ok := diffs.ProfileForKind(kind)
			require.True(t, ok)

			for run := range runs {
				fields := randomFields(rnd)
				current, updated := configs(fields...)

				changelog, err := diffs.NewBaseChecker(current, updated).GenerateDiff()
				require.NoError(t, err)

				extractor := extractorFor(t, kind, realRules, current)
				decisions := diffs.DecideRules(changelog, extractor, profile)
				verdicts := decisions.Verdicts

				assertChecksAgreeWithVerdicts(t, decisions, profile, run)
				assertChecksIndependent(t, changelog, extractor, profile, decisions, run)

				assert.Equal(t, leafPaths(diffs.ExpandMapChanges(changelog)), leafPaths(lo.Map(verdicts,
					func(v diffs.ChangeVerdict, _ int) r3diff.Change { return v.Change })),
					"run %d: one verdict per leaf change of %v", run, fields)

				for _, v := range verdicts {
					assertVerdictInvariants(t, v, profile, run)
				}

				// ReducerRulesByDiffs leaves values in the reducers of the
				// extractor, so a second run on the same extractor, and a run on a fresh
				// one, must still give the same facts.
				assert.Equal(t, factsOf(verdicts), factsOf(diffs.Verdicts(changelog, extractor, profile)),
					"run %d: a second run on the same extractor", run)
				assert.Equal(t, factsOf(verdicts), factsOf(diffs.Verdicts(changelog, extractorFor(t, kind, realRules, current), profile)),
					"run %d: a run on a fresh extractor", run)
			}
		})
	}
}

func assertVerdictInvariants(t *testing.T, v diffs.ChangeVerdict, profile diffs.KindProfile, run int) {
	t.Helper()

	path := v.Path()

	assert.True(t, len(v.Change.Path) >= len(v.Reported.Path) &&
		slices.Equal(v.Reported.Path, v.Change.Path[:len(v.Reported.Path)]),
		"run %d: %s is a leaf of the reported change %v", run, path, v.Reported.Path)

	// Blocked exactly when a preflight check fails, needs confirmation
	// exactly when an unsafe reducer matches and nothing blocks, none without any rule.
	assert.Equal(t, v.Immutable || v.Unsupported, v.Verdict == diffs.VerdictBlocked,
		"run %d: %s blocked exactly when immutable or unsupported", run, path)
	assert.Equal(t, hasUnsafeReducers(v) && v.Verdict != diffs.VerdictBlocked, v.Verdict == diffs.VerdictNeedsConfirmation,
		"run %d: %s needs confirmation exactly when an unsafe reducer matches and nothing blocks", run, path)

	if v.Verdict == diffs.VerdictNone {
		assert.Empty(t, v.Text, "run %d: %s has no text without a verdict", run, path)
		assert.False(t, hasUnsafeReducers(v), "run %d: %s has no unsafe reducer without a verdict", run, path)
	}

	assert.Contains(t, []diffs.Verdict{
		diffs.VerdictNone, diffs.VerdictMigration, diffs.VerdictNeedsConfirmation, diffs.VerdictBlocked,
	}, v.Verdict, "run %d: %s", run, path)

	for phase := range v.UnsafeReducers {
		assert.Contains(t, profile.UnsafeReducerPhases, phase, "run %d: %s unsafe reducers of a phase apply reads", run, path)
	}
}

func errorTexts(errs []error) []string {
	texts := make([]string, 0, len(errs))

	for _, err := range errs {
		texts = append(texts, err.Error())
	}

	return texts
}

// Apply takes its rule decisions from DecideRules, one rule check at a time.
func TestDecideRules_OneRuleCheckOfApply(t *testing.T) {
	t.Parallel()

	const (
		podCIDRViolation = "immutable value changed: path .spec.kubernetes.networking.podCIDR  " +
			"oldValue 172.16.128.0/17 newValue 10.42.0.0/16"
		nginxViolation = "unsupported value changed: currently, switching Ingress from single to dual nginx " +
			"is not supported. You need to first remove the current stack with type: none."
	)

	decide := func(t *testing.T, restrict func(diffs.KindProfile) diffs.KindProfile, fields ...field) diffs.RuleDecisions {
		t.Helper()

		current, updated := configs(fields...)

		changelog, err := diffs.NewBaseChecker(current, updated).GenerateDiff()
		require.NoError(t, err)

		profile, ok := diffs.ProfileForKind(kindImmutable)
		require.True(t, ok)

		return diffs.DecideRules(changelog, extractorFor(t, kindImmutable, realRules, current), restrict(profile))
	}

	changes := []field{
		{pathPodCIDR, "172.16.128.0/17", "10.42.0.0/16"},
		{pathNginxType, "single", "dual"},
	}

	t.Run("the whole profile gives the result of every rule check per phase", func(t *testing.T) {
		t.Parallel()

		got := decide(t, func(p diffs.KindProfile) diffs.KindProfile { return p }, changes...)

		assert.Equal(t, []string{podCIDRViolation}, errorTexts(got.ImmutableViolations))
		assert.Equal(t, []string{nginxViolation}, errorTexts(got.UnsupportedViolations))
		assert.Equal(t, []string{pathNginxType}, rules.Paths(got.UnsafeReducers[phaseDistribution]))
		assert.Empty(t, got.UnsafeReducers[phaseInfrastructure])

		require.Len(t, got.Verdicts, 2)
		assert.Equal(t, diffs.VerdictBlocked, verdictAt(t, got.Verdicts, pathPodCIDR).Verdict)
		assert.Equal(t, diffs.VerdictBlocked, verdictAt(t, got.Verdicts, pathNginxType).Verdict)
	})

	t.Run("the immutable check alone", func(t *testing.T) {
		t.Parallel()

		got := decide(t, diffs.KindProfile.OnlyImmutableCheck, changes...)

		assert.Equal(t, []string{podCIDRViolation}, errorTexts(got.ImmutableViolations))
		assert.Empty(t, got.UnsupportedViolations)
		assert.Empty(t, got.UnsafeReducers)
		assert.True(t, verdictAt(t, got.Verdicts, pathPodCIDR).Immutable)
		assert.Equal(t, diffs.VerdictNone, verdictAt(t, got.Verdicts, pathNginxType).Verdict)
	})

	t.Run("the unsupported check alone", func(t *testing.T) {
		t.Parallel()

		got := decide(t, diffs.KindProfile.OnlyUnsupportedCheck, changes...)

		assert.Equal(t, []string{nginxViolation}, errorTexts(got.UnsupportedViolations))
		assert.Empty(t, got.ImmutableViolations)
		assert.Empty(t, got.UnsafeReducers)
		assert.True(t, verdictAt(t, got.Verdicts, pathNginxType).Unsupported)
		assert.Equal(t, diffs.VerdictNone, verdictAt(t, got.Verdicts, pathPodCIDR).Verdict)
	})

	t.Run("the unsafe reducer check alone", func(t *testing.T) {
		t.Parallel()

		got := decide(t, diffs.KindProfile.OnlyUnsafeReducerCheck, changes...)

		assert.Equal(t, []string{pathNginxType}, rules.Paths(got.UnsafeReducers[phaseDistribution]))
		assert.Empty(t, got.ImmutableViolations)
		assert.Empty(t, got.UnsupportedViolations)
		assert.Equal(t, diffs.VerdictNeedsConfirmation, verdictAt(t, got.Verdicts, pathNginxType).Verdict)
		assert.Equal(t, diffs.VerdictNone, verdictAt(t, got.Verdicts, pathPodCIDR).Verdict)
	})

	t.Run("a whole object at an immutable path is one violation for all its leaves", func(t *testing.T) {
		t.Parallel()

		got := decide(t, diffs.KindProfile.OnlyImmutableCheck, etcdAdded())

		require.Len(t, got.Verdicts, 2)
		assert.Equal(
			t,
			[]string{
				"immutable value changed: path .spec.kubernetes.etcd  " +
					"oldValue <nil> newValue map[dataDir:/var/lib/etcd quota:8G]",
			},
			errorTexts(got.ImmutableViolations),
		)
	})

	t.Run("a rule matched by two changes is twice in the unsafe reducers", func(t *testing.T) {
		t.Parallel()

		got := decide(t, diffs.KindProfile.OnlyUnsafeReducerCheck, field{
			".spec.infrastructure.nodes",
			[]any{map[string]any{"hostname": "node-a"}, map[string]any{"hostname": "node-c"}},
			[]any{map[string]any{"hostname": "node-b"}, map[string]any{"hostname": "node-d"}},
		})

		assert.Equal(
			t,
			[]string{".spec.infrastructure.nodes.*.hostname", ".spec.infrastructure.nodes.*.hostname"},
			rules.Paths(got.UnsafeReducers[phaseInfrastructure]),
		)
	})
}

// What apply reads from the decisions is non-empty exactly
// when a verdict carries the same fact.
func assertChecksAgreeWithVerdicts(t *testing.T, decisions diffs.RuleDecisions, profile diffs.KindProfile, run int) {
	t.Helper()

	assert.Equal(t, len(decisions.ImmutableViolations) > 0,
		lo.SomeBy(decisions.Verdicts, func(v diffs.ChangeVerdict) bool { return v.Immutable }),
		"run %d: an immutable violation exactly when a verdict is immutable", run)
	assert.Equal(t, len(decisions.UnsupportedViolations) > 0,
		lo.SomeBy(decisions.Verdicts, func(v diffs.ChangeVerdict) bool { return v.Unsupported }),
		"run %d: an unsupported violation exactly when a verdict is unsupported", run)

	for _, phase := range profile.UnsafeReducerPhases {
		assert.Equal(t, len(decisions.UnsafeReducers[phase]) > 0,
			lo.SomeBy(decisions.Verdicts, func(v diffs.ChangeVerdict) bool { return len(v.UnsafeReducers[phase]) > 0 }),
			"run %d: unsafe reducers of %s exactly when a verdict has them", run, phase)
	}
}

// Apply runs the checks one at a time; each restricted profile gives,
// for its check, the same result as the whole profile, and nothing for the others.
func assertChecksIndependent(
	t *testing.T,
	changelog r3diff.Changelog,
	extractor rules.Extractor,
	profile diffs.KindProfile,
	whole diffs.RuleDecisions,
	run int,
) {
	t.Helper()

	immutable := diffs.DecideRules(changelog, extractor, profile.OnlyImmutableCheck())
	unsupported := diffs.DecideRules(changelog, extractor, profile.OnlyUnsupportedCheck())
	unsafe := diffs.DecideRules(changelog, extractor, profile.OnlyUnsafeReducerCheck())

	assert.Equal(t, errorTexts(whole.ImmutableViolations), errorTexts(immutable.ImmutableViolations), "run %d", run)
	assert.Empty(t, immutable.UnsupportedViolations, "run %d", run)
	assert.Empty(t, immutable.UnsafeReducers, "run %d", run)

	assert.Equal(t, errorTexts(whole.UnsupportedViolations), errorTexts(unsupported.UnsupportedViolations), "run %d", run)
	assert.Empty(t, unsupported.ImmutableViolations, "run %d", run)
	assert.Empty(t, unsupported.UnsafeReducers, "run %d", run)

	assert.Empty(t, unsafe.ImmutableViolations, "run %d", run)
	assert.Empty(t, unsafe.UnsupportedViolations, "run %d", run)

	for phase, want := range whole.UnsafeReducers {
		assert.Equal(t, rules.Paths(want), rules.Paths(unsafe.UnsafeReducers[phase]), "run %d: %s", run, phase)
	}

	for i, v := range whole.Verdicts {
		assert.Equal(t, v.Immutable, immutable.Verdicts[i].Immutable, "run %d: %s", run, v.Path())
		assert.False(t, immutable.Verdicts[i].Unsupported, "run %d: %s", run, v.Path())
		assert.Equal(t, v.Unsupported, unsupported.Verdicts[i].Unsupported, "run %d: %s", run, v.Path())
		assert.False(t, unsupported.Verdicts[i].Immutable, "run %d: %s", run, v.Path())
		assert.Equal(t, factsOf([]diffs.ChangeVerdict{v})[0].unsafe, factsOf(unsafe.Verdicts[i : i+1])[0].unsafe,
			"run %d: %s", run, v.Path())
	}
}
