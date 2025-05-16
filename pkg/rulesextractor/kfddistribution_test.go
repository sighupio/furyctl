// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package rulesextractor_test

import (
	"reflect"
	"testing"

	"github.com/r3labs/diff/v3"

	rules "github.com/sighupio/furyctl/pkg/rulesextractor"
	rules_extractor "github.com/sighupio/furyctl/pkg/rulesextractor"
)

func TestKFDBuilder_GetImmutableRules(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc  string
		Spec  *rules.Spec
		phase string
		want  []rules.Rule
	}{
		{
			desc:  "distribution - empty",
			Spec:  &rules.Spec{},
			phase: "distribution",
			want:  []rules.Rule{},
		},
		{
			desc: "distribution - not empty",
			Spec: &rules.Spec{
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

			builder := rules_extractor.DistroExtractor{
				Spec: *tC.Spec,
			}

			got := builder.GetImmutableRules(tC.phase)

			if !reflect.DeepEqual(got, tC.want) {
				t.Errorf("expected immutable rules to be %v, got: %v", tC.want, got)
			}
		})
	}
}

func TestKFDBuilder_FilterSafeImmutableRules(t *testing.T) {
	t.Parallel()

	var foo, bar any

	foo = "foo"
	bar = "bar"

	testCases := []struct {
		desc  string
		Spec  *rules.Spec
		rules []rules.Rule
		diffs diff.Changelog
		want  []rules.Rule
	}{
		{
			desc:  "empty rules",
			Spec:  &rules.Spec{},
			rules: []rules.Rule{},
			diffs: diff.Changelog{},
			want:  []rules.Rule{},
		},
		{
			desc: "no safe conditions",
			Spec: &rules.Spec{},
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
			desc: "matching safe conditions",
			Spec: &rules.Spec{},
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
			desc: "non-matching safe conditions",
			Spec: &rules.Spec{},
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

			builder := rules_extractor.DistroExtractor{
				Spec: *tC.Spec,
			}

			got := builder.FilterSafeImmutableRules(tC.rules, tC.diffs)

			if !reflect.DeepEqual(got, tC.want) {
				t.Errorf("expected filtered rules to be %v, got: %v", tC.want, got)
			}
		})
	}
}
