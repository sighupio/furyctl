// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package diffs_test

import (
	"testing"

	diffx "github.com/r3labs/diff/v3"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/pkg/diffs"
	rules "github.com/sighupio/furyctl/pkg/rulesextractor"
)

// An immutable violation on a sensitive path prints (hidden) in place of the values, and an object keeps its
// other values.
func TestBaseChecker_AssertImmutableViolations_HidesSensitiveValues(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc      string
		change    diffx.Change
		immutable string
		want      string
	}{
		{
			desc:      "sensitive leaf",
			change:    diffx.Change{Type: diffx.UPDATE, Path: []string{"spec", "kubernetes", "etcd", "password"}, From: "old-s3cr3t", To: "new-s3cr3t"},
			immutable: ".spec.kubernetes.etcd.password",
			want:      "immutable value changed: path .spec.kubernetes.etcd.password  oldValue (hidden) newValue (hidden)",
		},
		{
			desc:      "sensitive leaf added",
			change:    diffx.Change{Type: diffx.CREATE, Path: []string{"spec", "kubernetes", "etcd", "token"}, To: "s3cr3t"},
			immutable: ".spec.kubernetes.etcd.token",
			want:      "immutable value changed: path .spec.kubernetes.etcd.token  oldValue <nil> newValue (hidden)",
		},
		{
			desc: "object with a sensitive field",
			change: diffx.Change{
				Type: diffx.UPDATE, Path: []string{"spec", "kubernetes", "etcd"},
				From: map[string]any{"endpoint": "a", "clientKey": "old-s3cr3t"},
				To:   map[string]any{"endpoint": "b", "clientKey": "new-s3cr3t"},
			},
			immutable: ".spec.kubernetes.etcd",
			want:      "immutable value changed: path .spec.kubernetes.etcd  oldValue map[clientKey:(hidden) endpoint:a] newValue map[clientKey:(hidden) endpoint:b]",
		},
		{
			desc: "object without a sensitive field",
			change: diffx.Change{
				Type: diffx.UPDATE, Path: []string{"spec", "kubernetes", "etcd"},
				From: map[any]any{"endpoint": "a"}, To: map[any]any{"endpoint": "b"},
			},
			immutable: ".spec.kubernetes.etcd",
			want:      "immutable value changed: path .spec.kubernetes.etcd  oldValue map[endpoint:a] newValue map[endpoint:b]",
		},
	}

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			errs := diffs.NewBaseChecker(nil, nil).AssertImmutableViolations(diffx.Changelog{tC.change}, []string{tC.immutable})

			require.Len(t, errs, 1)
			require.Equal(t, tC.want, errs[0].Error())
			require.NotContains(t, errs[0].Error(), "s3cr3t")
		})
	}
}

// An unsupported violation on a sensitive path prints (hidden) in place of the values in its generic message.
func TestBaseChecker_AssertReducerUnsupportedViolations_HidesSensitiveValues(t *testing.T) {
	t.Parallel()

	cfg := func(token string) map[string]any {
		return map[string]any{"spec": map[string]any{"kubernetes": map[string]any{"advanced": map[string]any{"token": token}}}}
	}

	rule := rules.Rule{
		Path:        ".spec.kubernetes.advanced.token",
		Unsupported: &[]rules.Unsupported{{From: nil, To: nil, Reason: nil}},
	}

	checker := diffs.NewBaseChecker(cfg("old-s3cr3t"), cfg("new-s3cr3t"))

	changelog, err := checker.GenerateDiff()
	require.NoError(t, err)

	errs := checker.AssertReducerUnsupportedViolations(changelog, []rules.Rule{rule})

	require.Len(t, errs, 1)
	require.Equal(t,
		"unsupported value changed: changing .spec.kubernetes.advanced.token from (hidden) to (hidden) is not supported",
		errs[0].Error())
}

// A non-sensitive path keeps its values in the message, as before.
func TestBaseChecker_AssertReducerUnsupportedViolations_KeepsPlainValues(t *testing.T) {
	t.Parallel()

	cfg := func(kind string) map[string]any {
		return map[string]any{"spec": map[string]any{"kubernetes": map[string]any{"advanced": map[string]any{"kind": kind}}}}
	}

	rule := rules.Rule{
		Path:        ".spec.kubernetes.advanced.kind",
		Unsupported: &[]rules.Unsupported{{From: nil, To: nil, Reason: nil}},
	}

	checker := diffs.NewBaseChecker(cfg("ipvs"), cfg("none"))

	changelog, err := checker.GenerateDiff()
	require.NoError(t, err)

	errs := checker.AssertReducerUnsupportedViolations(changelog, []rules.Rule{rule})

	require.Len(t, errs, 1)
	require.Equal(t,
		"unsupported value changed: changing .spec.kubernetes.advanced.kind from ipvs to none is not supported",
		errs[0].Error())
}
