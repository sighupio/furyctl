// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package diffs_test

import (
	"fmt"
	"testing"

	diffx "github.com/r3labs/diff/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/pkg/diffs"
	rules "github.com/sighupio/furyctl/pkg/rulesextractor"
)

func TestBaseChecker_GenerateDiff(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc          string
		currentCfg    map[string]any
		newCfg        map[string]any
		expectedDiffs diffx.Changelog
		wantErr       bool
		wantErrMsg    string
	}{
		{
			desc: "no diffs",
			currentCfg: map[string]any{
				"foo": "bar",
			},
			newCfg: map[string]any{
				"foo": "bar",
			},
			expectedDiffs: diffx.Changelog{},
			wantErr:       false,
		},
		{
			desc: "diffs found - simple",
			currentCfg: map[string]any{
				"foo": "bar",
			},
			newCfg: map[string]any{
				"foo": "baz",
			},
			expectedDiffs: diffx.Changelog{
				{
					Type: diffx.UPDATE,
					Path: []string{"foo"},
					From: "bar",
					To:   "baz",
				},
			},
			wantErr: false,
		},
		{
			desc:       "diffs found - current config nil",
			currentCfg: nil,
			newCfg: map[string]any{
				"foo": "baz",
			},
			expectedDiffs: diffx.Changelog{
				{
					Type: diffx.CREATE,
					Path: []string{"foo"},
					To:   "baz",
				},
			},
			wantErr: false,
		},
		{
			desc: "diffs found - new config nil",
			currentCfg: map[string]any{
				"foo": "bar",
			},
			newCfg: nil,
			expectedDiffs: diffx.Changelog{
				{
					Type: diffx.DELETE,
					Path: []string{"foo"},
					From: "bar",
				},
			},
			wantErr: false,
		},
		{
			desc:          "no diffs - nil",
			currentCfg:    nil,
			newCfg:        nil,
			expectedDiffs: diffx.Changelog{},
			wantErr:       false,
		},
	}

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			checker := diffs.NewBaseChecker(tC.currentCfg, tC.newCfg)

			diffs, err := checker.GenerateDiff()
			if tC.wantErr {
				require.Error(t, err)
				assert.Equal(t, tC.wantErrMsg, err.Error())
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, tC.expectedDiffs, diffs)
		})
	}
}

func TestBaseChecker_AssertImmutableViolations(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc           string
		diffs          diffx.Changelog
		immutablePaths []string
		expectedErrs   []error
	}{
		{
			desc:           "no diffs",
			diffs:          diffx.Changelog{},
			immutablePaths: []string{},
			expectedErrs:   []error{},
		},
		{
			desc: "no immutable paths",
			diffs: diffx.Changelog{
				{
					Type: diffx.CREATE,
					Path: []string{"foo"},
					To:   1,
				},
			},
			immutablePaths: []string{},
			expectedErrs:   []error{},
		},
		{
			desc: "immutable paths found",
			diffs: diffx.Changelog{
				{
					Type: diffx.UPDATE,
					Path: []string{"spec", "foo", "2", "bar"},
					From: "bar",
					To:   "baz",
				},
				{
					Type: diffx.UPDATE,
					Path: []string{"spec", "bar", "baz"},
					From: "baz",
					To:   "bar",
				},
			},
			immutablePaths: []string{".spec.foo.*.bar", ".spec.bar.baz", ".spec.test.key"},
			expectedErrs: []error{
				fmt.Errorf("immutable value changed: path .spec.foo.2.bar  oldValue bar newValue baz"),
				fmt.Errorf("immutable value changed: path .spec.bar.baz  oldValue baz newValue bar"),
			},
		},
	}

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			checker := diffs.NewBaseChecker(nil, nil)

			errs := checker.AssertImmutableViolations(tC.diffs, tC.immutablePaths)

			assert.Len(t, errs, len(tC.expectedErrs))

			for i, err := range errs {
				assert.Equal(t, tC.expectedErrs[i].Error(), err.Error())
			}
		})
	}
}

// onpremKubeCfg builds a minimal on-prem-like config; pass nil to omit the
// kubeProxy object entirely (so the parent is absent).
func onpremKubeCfg(kubeProxy map[string]any) map[string]any {
	advanced := map[string]any{}
	if kubeProxy != nil {
		advanced["kubeProxy"] = kubeProxy
	}

	return map[string]any{
		"spec": map[string]any{
			"kubernetes": map[string]any{
				"advanced": advanced,
			},
		},
	}
}

// TestBaseChecker_AssertReducerUnsupportedViolations_WithoutReducers guards the
// fix for unsupported transitions declared on a rule that has NO reducers
// (e.g. .spec.kubernetes.advanced.kubeProxy.type), including the nil -> value
// and value -> nil cases where the parent object is added/removed wholesale.
func TestBaseChecker_AssertReducerUnsupportedViolations_WithoutReducers(t *testing.T) {
	t.Parallel()

	// Rule with only `unsupported` (no reducers) — the case the fix enables.
	rule := rules.Rule{
		Path: ".spec.kubernetes.advanced.kubeProxy.type",
		Unsupported: &[]rules.Unsupported{
			{To: new(any("none")), Reason: new("disabling kube-proxy on an existing cluster is not supported")},
			{From: new(any("none")), Reason: new("enabling kube-proxy where it was never installed is not supported")},
		},
	}

	testCases := []struct {
		desc        string
		current     map[string]any
		next        map[string]any
		wantBlocked bool
	}{
		{"nil -> none (kubeProxy object absent before)", onpremKubeCfg(nil), onpremKubeCfg(map[string]any{"type": "none"}), true},
		{"nil -> none (kubeProxy present, type added)", onpremKubeCfg(map[string]any{}), onpremKubeCfg(map[string]any{"type": "none"}), true},
		{"ipvs -> none", onpremKubeCfg(map[string]any{"type": "ipvs"}), onpremKubeCfg(map[string]any{"type": "none"}), true},
		{"none -> ipvs", onpremKubeCfg(map[string]any{"type": "none"}), onpremKubeCfg(map[string]any{"type": "ipvs"}), true},
		{"none -> nil (kubeProxy object removed)", onpremKubeCfg(map[string]any{"type": "none"}), onpremKubeCfg(nil), true},
		{"ipvs -> nftables (allowed)", onpremKubeCfg(map[string]any{"type": "ipvs"}), onpremKubeCfg(map[string]any{"type": "nftables"}), false},
		{"no change", onpremKubeCfg(map[string]any{"type": "ipvs"}), onpremKubeCfg(map[string]any{"type": "ipvs"}), false},
	}

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			checker := diffs.NewBaseChecker(tC.current, tC.next)

			changelog, err := checker.GenerateDiff()
			require.NoError(t, err)

			errs := checker.AssertReducerUnsupportedViolations(changelog, []rules.Rule{rule})

			if tC.wantBlocked {
				assert.NotEmpty(t, errs, "expected an unsupported-transition violation")
			} else {
				assert.Empty(t, errs, "expected no violation")
			}
		})
	}
}

// TestExpandMapChanges verifies that a wholesale object create/delete (a change
// whose value is a nested map) is expanded into one change per leaf, so that
// leaf-targeted rules/reducers match nil->value and value->nil transitions.
func TestExpandMapChanges(t *testing.T) {
	t.Parallel()

	t.Run("parent created -> per-leaf change with nil From", func(t *testing.T) {
		t.Parallel()

		in := diffx.Changelog{
			{Type: "create", Path: []string{"spec", "kubernetes", "advanced", "kubeProxy"}, From: nil, To: map[string]any{"type": "nftables"}},
		}

		out := diffs.ExpandMapChanges(in)

		require.Len(t, out, 1)
		assert.Equal(t, []string{"spec", "kubernetes", "advanced", "kubeProxy", "type"}, out[0].Path)
		assert.Nil(t, out[0].From)
		assert.Equal(t, "nftables", out[0].To)
	})

	t.Run("parent deleted -> per-leaf change with nil To", func(t *testing.T) {
		t.Parallel()

		in := diffx.Changelog{
			{Type: "delete", Path: []string{"spec", "kubernetes", "advanced", "kubeProxy"}, From: map[string]any{"type": "ipvs"}, To: nil},
		}

		out := diffs.ExpandMapChanges(in)

		require.Len(t, out, 1)
		assert.Equal(t, []string{"spec", "kubernetes", "advanced", "kubeProxy", "type"}, out[0].Path)
		assert.Equal(t, "ipvs", out[0].From)
		assert.Nil(t, out[0].To)
	})

	t.Run("leaf change is returned unchanged", func(t *testing.T) {
		t.Parallel()

		in := diffx.Changelog{
			{Type: "update", Path: []string{"spec", "kubernetes", "advanced", "kubeProxy", "type"}, From: "ipvs", To: "nftables"},
		}

		out := diffs.ExpandMapChanges(in)

		require.Len(t, out, 1)
		assert.Equal(t, in[0], out[0])
	})
}

func TestBaseChecker_DiffToString(t *testing.T) {
	t.Parallel()

	checker := diffs.NewBaseChecker(nil, nil)

	t.Run("the apply debug text hides the same values", func(t *testing.T) {
		t.Parallel()

		got := checker.DiffToString(diffx.Changelog{
			{
				Type: diffx.UPDATE,
				Path: []string{"spec", "infrastructure", "loadBalancers", "keepalived", "passphrase"},
				From: "sentinel-old-4f1c",
				To:   "sentinel-new-9a7e",
			},
			{
				Type: diffx.CREATE,
				Path: []string{"spec", "distribution", "modules", "auth", "dex"},
				From: nil,
				To:   map[string]any{"clientSecret": "sentinel-nested-22b0", "issuer": "https://dex.example.test"},
			},
		})

		assert.Equal(
			t,
			".spec.infrastructure.loadBalancers.keepalived.passphrase: (hidden) -> (hidden)\n"+
				".spec.distribution.modules.auth.dex: <nil> -> map[clientSecret:(hidden) issuer:https://dex.example.test]\n",
			got,
		)
		assert.NotContains(t, got, "sentinel-")
	})

	t.Run("a side where a sensitive field does not exist is not hidden", func(t *testing.T) {
		t.Parallel()

		got := checker.DiffToString(diffx.Changelog{
			{Type: diffx.CREATE, Path: []string{"spec", "plugins", "apiToken"}, From: nil, To: "sentinel-added-50ee"},
			{Type: diffx.DELETE, Path: []string{"spec", "plugins", "sshKey"}, From: "sentinel-removed-60ff", To: nil},
		})

		assert.Equal(
			t,
			".spec.plugins.apiToken: <nil> -> (hidden)\n.spec.plugins.sshKey: (hidden) -> <nil>\n",
			got,
		)
	})

	t.Run("a change without secrets is printed as before", func(t *testing.T) {
		t.Parallel()

		got := checker.DiffToString(diffx.Changelog{
			{Type: diffx.UPDATE, Path: []string{"spec", "distribution", "modules", "logging", "type"}, From: "none", To: "loki"},
			{Type: diffx.CREATE, Path: []string{"spec", "kubernetes", "advanced", "kubeProxy"}, From: nil, To: map[string]any{"type": "nftables"}},
			{Type: diffx.UPDATE, Path: []string{"spec", "kubernetes", "nodes", "0", "replicas"}, From: 1, To: 3},
		})

		assert.Equal(
			t,
			".spec.distribution.modules.logging.type: none -> loki\n"+
				".spec.kubernetes.advanced.kubeProxy: <nil> -> map[type:nftables]\n"+
				".spec.kubernetes.nodes.0.replicas: 1 -> 3\n",
			got,
		)
	})
}

// An object with no key has no leaf to expand into: its change stays as it is, so
// the table still has a row for it and the rule checks still see it.
func TestExpandMapChanges_EmptyObject(t *testing.T) {
	t.Parallel()

	path := []string{"spec", "kubernetes", "advanced"}

	testCases := []struct {
		name   string
		change diffx.Change
	}{
		{
			name:   "an added object without keys stays one change",
			change: diffx.Change{Type: diffx.CREATE, Path: path, From: nil, To: map[string]any{}},
		},
		{
			name:   "a removed object without keys stays one change",
			change: diffx.Change{Type: diffx.DELETE, Path: path, From: map[string]any{}, To: nil},
		},
	}

	for _, tC := range testCases {
		t.Run(tC.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, diffx.Changelog{tC.change}, diffs.ExpandMapChanges(diffx.Changelog{tC.change}))
		})
	}
}
