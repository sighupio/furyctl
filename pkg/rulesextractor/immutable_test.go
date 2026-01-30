// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package rulesextractor_test

import (
	"reflect"
	"testing"

	"github.com/r3labs/diff/v3"

	"github.com/sighupio/furyctl/pkg/rulesextractor"
)

func TestImmutableBuilder_GetImmutableRules(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc               string
		ImmutableRulesSpec *rulesextractor.Spec
		phase              string
		want               []rulesextractor.Rule
	}{
		{
			desc:               "infrastucture - empty",
			ImmutableRulesSpec: &rulesextractor.Spec{},
			phase:              "infrastructure",
			want:               []rulesextractor.Rule{},
		},
		{
			desc:               "kubernetes - empty",
			ImmutableRulesSpec: &rulesextractor.Spec{},
			phase:              "kubernetes",
			want:               []rulesextractor.Rule{},
		},
		{
			desc:               "distribution - empty",
			ImmutableRulesSpec: &rulesextractor.Spec{},
			phase:              "distribution",
			want:               []rulesextractor.Rule{},
		},
		{
			desc: "infrastructure - not empty",
			ImmutableRulesSpec: &rulesextractor.Spec{
				Infrastructure: &[]rulesextractor.Rule{
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
			want: []rulesextractor.Rule{
				{
					Path:      "foo",
					Immutable: true,
				},
			},
		},
		{
			desc: "kubernetes - not empty",
			ImmutableRulesSpec: &rulesextractor.Spec{
				Kubernetes: &[]rulesextractor.Rule{
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
			want: []rulesextractor.Rule{
				{
					Path:      "foo",
					Immutable: true,
				},
			},
		},
		{
			desc: "distribution - not empty",
			ImmutableRulesSpec: &rulesextractor.Spec{
				Distribution: &[]rulesextractor.Rule{
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
			want: []rulesextractor.Rule{
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

			builder := rulesextractor.ImmutableExtractor{
				Spec: *tC.ImmutableRulesSpec,
			}

			got := builder.GetImmutableRules(tC.phase)

			if !reflect.DeepEqual(got, tC.want) {
				t.Errorf("expected immutable rules to be %v, got: %v", tC.want, got)
			}
		})
	}
}

func TestImmutableBuilder_FilterSafeImmutableRules(t *testing.T) {
	t.Parallel()

	var foo, bar any

	foo = "foo"
	bar = "bar"

	testCases := []struct {
		desc               string
		ImmutableRulesSpec *rulesextractor.Spec
		rules              []rulesextractor.Rule
		diffs              diff.Changelog
		want               []rulesextractor.Rule
	}{
		{
			desc:               "empty rules",
			ImmutableRulesSpec: &rulesextractor.Spec{},
			rules:              []rulesextractor.Rule{},
			diffs:              diff.Changelog{},
			want:               []rulesextractor.Rule{},
		},
		{
			desc:               "no safe conditions",
			ImmutableRulesSpec: &rulesextractor.Spec{},
			rules: []rulesextractor.Rule{
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
			want: []rulesextractor.Rule{
				{
					Path:      "foo",
					Immutable: true,
				},
			},
		},
		{
			desc:               "matching safe conditions",
			ImmutableRulesSpec: &rulesextractor.Spec{},
			rules: []rulesextractor.Rule{
				{
					Path:      ".foo",
					Immutable: true,
					Safe: &[]rulesextractor.Safe{
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
			want: []rulesextractor.Rule{},
		},
		{
			desc:               "non-matching safe conditions",
			ImmutableRulesSpec: &rulesextractor.Spec{},
			rules: []rulesextractor.Rule{
				{
					Path:      ".foo",
					Immutable: true,
					Safe: &[]rulesextractor.Safe{
						{
							From: &foo,
							To:   &foo, // Doesn't match the diff.
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
			want: []rulesextractor.Rule{
				{
					Path:      ".foo",
					Immutable: true,
					Safe: &[]rulesextractor.Safe{
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

			builder := rulesextractor.ImmutableExtractor{
				Spec: *tC.ImmutableRulesSpec,
			}

			got := builder.FilterSafeImmutableRules(tC.rules, tC.diffs)

			if !reflect.DeepEqual(got, tC.want) {
				t.Errorf("expected filtered rules to be %v, got: %v", tC.want, got)
			}
		})
	}
}
