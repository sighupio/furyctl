// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package rulesextractor_test

import (
	"reflect"
	"testing"

	"github.com/r3labs/diff/v3"

	eksrules "github.com/sighupio/furyctl/pkg/rulesextractor"
	rules "github.com/sighupio/furyctl/pkg/rulesextractor"
)

func TestEKSBuilder_GetImmutableRules(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc         string
		eksRulesSpec *rules.Spec
		phase        string
		want         []rules.Rule
	}{
		{
			desc:         "infrastructure - empty",
			eksRulesSpec: &rules.Spec{},
			phase:        "infrastructure",
			want:         []rules.Rule{},
		},
		{
			desc:         "kubernetes - empty",
			eksRulesSpec: &rules.Spec{},
			phase:        "kubernetes",
			want:         []rules.Rule{},
		},
		{
			desc:         "distribution - empty",
			eksRulesSpec: &rules.Spec{},
			phase:        "distribution",
			want:         []rules.Rule{},
		},
		{
			desc: "infrastructure - not empty",
			eksRulesSpec: &rules.Spec{
				Infrastructure: &[]rules.Rule{
					{
						Path:      "foo",
						Immutable: true,
					},
					{
						Path:      "bar",
						Immutable: false,
					},
				},
			},
			phase: "infrastructure",
			want: []rules.Rule{
				{
					Path:      "foo",
					Immutable: true,
				},
			},
		},
		{
			desc: "kubernetes - not empty",
			eksRulesSpec: &rules.Spec{
				Kubernetes: &[]rules.Rule{
					{
						Path:      "foo",
						Immutable: true,
					},
					{
						Path:      "bar",
						Immutable: false,
					},
				},
			},
			phase: "kubernetes",
			want: []rules.Rule{
				{
					Path:      "foo",
					Immutable: true,
				},
			},
		},
		{
			desc: "distribution - not empty",
			eksRulesSpec: &rules.Spec{
				Distribution: &[]rules.Rule{
					{
						Path:      "foo",
						Immutable: true,
					},
					{
						Path:      "bar",
						Immutable: false,
					},
				},
			},
			phase: "distribution",
			want: []rules.Rule{
				{
					Path:      "foo",
					Immutable: true,
				},
			},
		},
	}

	for _, tC := range testCases {
		tC := tC

		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			builder := eksrules.EKSExtractor{
				Spec: *tC.eksRulesSpec,
			}

			got := builder.GetImmutableRules(tC.phase)

			if !reflect.DeepEqual(got, tC.want) {
				t.Errorf("expected immutable rules to be %v, got: %v", tC.want, got)
			}
		})
	}
}

func TestEKSBuilder_FilterSafeImmutableRules(t *testing.T) {
	t.Parallel()

	var foo, bar any

	foo = "foo"
	bar = "bar"

	testCases := []struct {
		desc         string
		eksRulesSpec *rules.Spec
		rules        []rules.Rule
		diffs        diff.Changelog
		want         []rules.Rule
	}{
		{
			desc:         "empty rules",
			eksRulesSpec: &rules.Spec{},
			rules:        []rules.Rule{},
			diffs:        diff.Changelog{},
			want:         []rules.Rule{},
		},
		{
			desc:         "no safe conditions",
			eksRulesSpec: &rules.Spec{},
			rules: []rules.Rule{
				{
					Path:      "foo",
					Immutable: true,
				},
			},
			diffs: diff.Changelog{
				{
					Path: []string{"foo"},
					From: "foo",
					To:   "bar",
				},
			},
			want: []rules.Rule{
				{
					Path:      "foo",
					Immutable: true,
				},
			},
		},
		{
			desc:         "matching safe conditions",
			eksRulesSpec: &rules.Spec{},
			rules: []rules.Rule{
				{
					Path:      ".foo",
					Immutable: true,
					Safe: &[]rules.Safe{
						{
							From: &foo,
							To:   &bar,
						},
					},
				},
			},
			diffs: diff.Changelog{
				{
					Path: []string{"foo"},
					From: "foo",
					To:   "bar",
				},
			},
			want: []rules.Rule{},
		},
		{
			desc:         "non-matching safe conditions",
			eksRulesSpec: &rules.Spec{},
			rules: []rules.Rule{
				{
					Path:      ".foo",
					Immutable: true,
					Safe: &[]rules.Safe{
						{
							From: &foo,
							To:   &foo, // Doesn't match the diff
						},
					},
				},
			},
			diffs: diff.Changelog{
				{
					Path: []string{"foo"},
					From: "foo",
					To:   "bar",
				},
			},
			want: []rules.Rule{
				{
					Path:      ".foo",
					Immutable: true,
					Safe: &[]rules.Safe{
						{
							From: &foo,
							To:   &foo,
						},
					},
				},
			},
		},
	}

	for _, tC := range testCases {
		tC := tC

		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			builder := eksrules.EKSExtractor{
				Spec: *tC.eksRulesSpec,
			}

			got := builder.FilterSafeImmutableRules(tC.rules, tC.diffs)

			if !reflect.DeepEqual(got, tC.want) {
				t.Errorf("expected filtered rules to be %v, got: %v", tC.want, got)
			}
		})
	}
}
