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
)

func TestNewBaseExtractor(t *testing.T) {
	t.Parallel()

	x := rules.NewBaseExtractor(rules.Spec{})

	if x == nil {
		t.Errorf("expected not nil, got %v", x)
	}
}

func TestBaseExtractor_GetImmutables(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		spec rules.Spec
		want []string
	}{
		{
			name: "should return empty slice if no rules",
			spec: rules.Spec{},
			want: nil,
		},
		{
			name: "return immutables from infrastructure rules",
			spec: rules.Spec{
				Infrastructure: &[]rules.Rule{
					{
						Path:      ".foo",
						Immutable: true,
					},
					{
						Path:      ".bar",
						Immutable: false,
					},
				},
			},
			want: []string{".foo"},
		},
		{
			name: "return immutables from kubernetes rules",
			spec: rules.Spec{
				Kubernetes: &[]rules.Rule{
					{
						Path:      ".foo",
						Immutable: true,
					},
					{
						Path:      ".bar",
						Immutable: false,
					},
				},
			},
			want: []string{".foo"},
		},
		{
			name: "return immutables from distribution rules",
			spec: rules.Spec{
				Distribution: &[]rules.Rule{
					{
						Path:      ".foo",
						Immutable: true,
					},
					{
						Path:      ".bar",
						Immutable: false,
					},
				},
			},
			want: []string{".foo"},
		},
		{
			name: "return immutables from all rules",
			spec: rules.Spec{
				Infrastructure: &[]rules.Rule{
					{
						Path:      ".foo",
						Immutable: true,
					},
					{
						Path:      ".bar",
						Immutable: false,
					},
				},
				Kubernetes: &[]rules.Rule{
					{
						Path:      ".foo2",
						Immutable: true,
					},
					{
						Path:      ".bar2",
						Immutable: false,
					},
				},
				Distribution: &[]rules.Rule{
					{
						Path:      ".foo3",
						Immutable: true,
					},
					{
						Path:      ".bar3",
						Immutable: false,
					},
				},
			},
			want: []string{".foo", ".foo2", ".foo3"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			x := rules.NewBaseExtractor(tc.spec)

			got := x.GetImmutables("")

			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("expected %v, got %v", tc.want, got)
			}
		})
	}
}

func TestBaseExtractor_GetReducers(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		spec rules.Spec
		want []rules.Rule
	}{
		{
			name: "should return empty slice if no rules",
			spec: rules.Spec{},
			want: nil,
		},
		{
			name: "return reducers from infrastructure rules",
			spec: rules.Spec{
				Infrastructure: &[]rules.Rule{
					{
						Path: ".foo",
						Reducers: &[]rules.Reducer{
							{
								From: "foo",
								To:   "bar",
							},
						},
					},
					{
						Path: "bar",
					},
				},
			},
			want: []rules.Rule{
				{
					Path: ".foo",
					Reducers: &[]rules.Reducer{
						{
							From: "foo",
							To:   "bar",
						},
					},
				},
			},
		},
		{
			name: "return reducers from kubernetes rules",
			spec: rules.Spec{
				Kubernetes: &[]rules.Rule{
					{
						Path: ".foo",
						Reducers: &[]rules.Reducer{
							{
								From: "foo",
								To:   "bar",
							},
						},
					},
					{
						Path: "bar",
					},
				},
			},
			want: []rules.Rule{
				{
					Path: ".foo",
					Reducers: &[]rules.Reducer{
						{
							From: "foo",
							To:   "bar",
						},
					},
				},
			},
		},
		{
			name: "return reducers from distribution rules",
			spec: rules.Spec{
				Distribution: &[]rules.Rule{
					{
						Path: ".foo",
						Reducers: &[]rules.Reducer{
							{
								From: "foo",
								To:   "bar",
							},
						},
					},
					{
						Path: "bar",
					},
				},
			},
			want: []rules.Rule{
				{
					Path: ".foo",
					Reducers: &[]rules.Reducer{
						{
							From: "foo",
							To:   "bar",
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			x := rules.NewBaseExtractor(tc.spec)

			got := x.GetReducers("")

			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("expected %v, got %v", tc.want, got)
			}
		})
	}
}

func TestBaseExtractor_ReducerRulesByDiffs(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		rules []rules.Rule
		diffs diff.Changelog
		want  []rules.Rule
	}{
		{
			name:  "should return empty slice if no rules",
			rules: []rules.Rule{},
			diffs: diff.Changelog{
				{
					Type: diff.CREATE,
					Path: []string{"foo"},
				},
			},
			want: []rules.Rule{},
		},
		{
			name: "should handle nil diffs",
			rules: []rules.Rule{
				{
					Path: ".foo",
					Reducers: &[]rules.Reducer{
						{
							From: "foo",
							To:   "bar",
						},
					},
				},
			},
			diffs: nil,
			want:  []rules.Rule{},
		},
		{
			name: "should return rules if a diff matches",
			rules: []rules.Rule{
				{
					Path: ".foo",
					Reducers: &[]rules.Reducer{
						{
							From: "foo",
							To:   "bar",
						},
					},
				},
				{
					Path: ".bar",
					Reducers: &[]rules.Reducer{
						{
							From: "bar",
							To:   "foo",
						},
					},
				},
				{
					Path: ".baz",
					Reducers: &[]rules.Reducer{
						{
							From: "baz",
							To:   "foo",
						},
					},
				},
			},
			diffs: diff.Changelog{
				{
					Type: diff.CREATE,
					Path: []string{"foo"},
					From: "foo",
					To:   "bar",
				},
				{
					Type: diff.CREATE,
					Path: []string{"bar"},
					From: "bar",
					To:   "foo",
				},
			},
			want: []rules.Rule{
				{
					Path: ".foo",
					Reducers: &[]rules.Reducer{
						{
							From: "foo",
							To:   "bar",
						},
					},
				},
				{
					Path: ".bar",
					Reducers: &[]rules.Reducer{
						{
							From: "bar",
							To:   "foo",
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			x := rules.NewBaseExtractor(rules.Spec{})

			got := x.ReducerRulesByDiffs(tc.rules, tc.diffs)

			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("expected %v, got %v", tc.want, got)
			}
		})
	}
}

func TestBaseExtractor_UnsupportedReducerRulesByDiffs(t *testing.T) {
	t.Parallel()

	var foo, bar, baz any

	foo = "foo"
	bar = "bar"
	baz = "baz"

	testCases := []struct {
		name  string
		rules []rules.Rule
		diffs diff.Changelog
		want  []rules.Rule
	}{
		{
			name:  "should return empty slice if no rules",
			rules: []rules.Rule{},
			diffs: diff.Changelog{
				{
					Type: diff.CREATE,
					Path: []string{"foo"},
				},
			},
			want: []rules.Rule{},
		},
		{
			name: "should handle nil diffs",
			rules: []rules.Rule{
				{
					Path: ".foo",
					Reducers: &[]rules.Reducer{
						{
							From: "foo",
							To:   "bar",
						},
					},
				},
			},
			diffs: nil,
			want:  []rules.Rule{},
		},
		{
			name: "should return rules if a diff matches",
			rules: []rules.Rule{
				{
					Path: ".foo",
					Reducers: &[]rules.Reducer{
						{
							From: "foo",
							To:   "bar",
						},
					},
					Unsupported: &[]rules.Unsupported{
						{
							From: &foo,
							To:   &bar,
						},
					},
				},
				{
					Path: ".bar",
					Reducers: &[]rules.Reducer{
						{
							From: "bar",
							To:   "foo",
						},
					},
					Unsupported: &[]rules.Unsupported{
						{
							From: &bar,
							To:   &foo,
						},
					},
				},
				{
					Path: ".baz",
					Reducers: &[]rules.Reducer{
						{
							From: "baz",
							To:   "foo",
						},
					},
					Unsupported: &[]rules.Unsupported{
						{
							From: &baz,
							To:   &foo,
						},
					},
				},
			},
			diffs: diff.Changelog{
				{
					Type: diff.CREATE,
					Path: []string{"foo"},
					From: "foo",
					To:   "bar",
				},
				{
					Type: diff.CREATE,
					Path: []string{"bar"},
					From: "bar",
					To:   "foo",
				},
			},
			want: []rules.Rule{
				{
					Path: ".foo",
					Reducers: &[]rules.Reducer{
						{
							From: "foo",
							To:   "bar",
						},
					},
					Unsupported: &[]rules.Unsupported{
						{
							From: &foo,
							To:   &bar,
						},
					},
				},
				{
					Path: ".bar",
					Reducers: &[]rules.Reducer{
						{
							From: "bar",
							To:   "foo",
						},
					},
					Unsupported: &[]rules.Unsupported{
						{
							From: &bar,
							To:   &foo,
						},
					},
				},
			},
		},
		{
			name: "should return rules if a diff matches - complex diffs",
			rules: []rules.Rule{
				{
					Path: ".foo",
					Reducers: &[]rules.Reducer{
						{
							From: "foo",
							To:   "bar",
						},
					},
					Unsupported: &[]rules.Unsupported{
						{
							From: &foo,
						},
					},
				},
				{
					Path: ".bar",
					Reducers: &[]rules.Reducer{
						{
							From: "bar",
							To:   "foo",
						},
					},
					Unsupported: &[]rules.Unsupported{
						{
							To: &foo,
						},
					},
				},
				{
					Path: ".baz",
					Reducers: &[]rules.Reducer{
						{
							From: "baz",
							To:   "foo",
						},
					},
					Unsupported: &[]rules.Unsupported{},
				},
			},
			diffs: diff.Changelog{
				{
					Type: diff.CREATE,
					Path: []string{"foo"},
					From: "foo",
					To:   "bar",
				},
				{
					Type: diff.CREATE,
					Path: []string{"bar"},
					From: "bar",
					To:   "foo",
				},
				{
					Type: diff.CREATE,
					Path: []string{"baz"},
					From: "baz",
					To:   "foo",
				},
			},
			want: []rules.Rule{
				{
					Path: ".foo",
					Reducers: &[]rules.Reducer{
						{
							From: "foo",
							To:   "bar",
						},
					},
					Unsupported: &[]rules.Unsupported{
						{
							From: &foo,
						},
					},
				},
				{
					Path: ".bar",
					Reducers: &[]rules.Reducer{
						{
							From: "bar",
							To:   "foo",
						},
					},
					Unsupported: &[]rules.Unsupported{
						{
							To: &foo,
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			x := rules.NewBaseExtractor(rules.Spec{})

			got := x.UnsupportedReducerRulesByDiffs(tc.rules, tc.diffs)

			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("expected %v, got %v", tc.want, got)
			}
		})
	}
}

func TestBaseExtractor_ExtractImmutablesFromRules(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		rules []rules.Rule
		want  []string
	}{
		{
			name:  "should return empty slice if no rules",
			rules: []rules.Rule{},
			want:  []string{},
		},
		{
			name: "should return empty slice if no immutables",
			rules: []rules.Rule{
				{
					Path: ".foo",
				},
			},
			want: []string{},
		},
		{
			name: "should return immutables",
			rules: []rules.Rule{
				{
					Path:      ".foo",
					Immutable: true,
				},
				{
					Path:      ".bar",
					Immutable: false,
				},
			},
			want: []string{".foo"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			x := rules.NewBaseExtractor(rules.Spec{})

			got := x.ExtractImmutablesFromRules(tc.rules)

			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("expected %v, got %v", tc.want, got)
			}
		})
	}
}

func TestBaseExtractor_UnsafeReducerRulesByDiffs(t *testing.T) {
	t.Parallel()

	var foo, bar, none, loki, tsdbDate any

	foo = "foo"
	bar = "bar"
	none = "none"
	loki = "loki"
	tsdbDate = "2023-01-01"

	stringPtr := func(s string) *string {
		return &s
	}

	testCases := []struct {
		name  string
		rules []rules.Rule
		diffs diff.Changelog
		want  []rules.Rule
	}{
		{
			name:  "should return empty slice if no rules",
			rules: []rules.Rule{},
			diffs: diff.Changelog{
				{
					Type: diff.CREATE,
					Path: []string{"foo"},
				},
			},
			want: []rules.Rule{},
		},
		{
			name: "should handle nil diffs",
			rules: []rules.Rule{
				{
					Path: ".foo",
					Reducers: &[]rules.Reducer{
						{
							From: &foo,
							To:   &bar,
						},
					},
					Safe: &[]rules.Safe{
						{
							From: &foo,
							To:   &bar,
						},
					},
				},
			},
			diffs: nil,
			want:  []rules.Rule{},
		},
		{
			name: "should handle nil safe",
			rules: []rules.Rule{
				{
					Path: ".foo",
					Reducers: &[]rules.Reducer{
						{
							From: "foo",
							To:   "bar",
						},
					},
					Safe: nil,
				},
			},
			diffs: diff.Changelog{
				{
					Type: diff.CREATE,
					Path: []string{"foo"},
					From: "foo",
					To:   "bar",
				},
			},
			want: []rules.Rule{
				{
					Path: ".foo",
					Reducers: &[]rules.Reducer{
						{
							From: "foo",
							To:   "bar",
						},
					},
					Safe: nil,
				},
			},
		},
		{
			name: "should handle empty safe",
			rules: []rules.Rule{
				{
					Path: ".foo",
					Reducers: &[]rules.Reducer{
						{
							From: "foo",
							To:   "bar",
						},
					},
					Safe: &[]rules.Safe{{}},
				},
			},
			diffs: diff.Changelog{
				{
					Type: diff.CREATE,
					Path: []string{"foo"},
					From: "foo",
					To:   "bar",
				},
			},
			want: []rules.Rule{},
		},
		{
			name: "should handle nil from in safe",
			rules: []rules.Rule{
				{
					Path: ".foo",
					Reducers: &[]rules.Reducer{
						{
							From: "foo",
							To:   "bar",
						},
						{
							From: "baz",
							To:   "bar",
						},
					},
					Safe: &[]rules.Safe{
						{
							To: &bar,
						},
					},
				},
			},
			diffs: diff.Changelog{
				{
					Type: diff.CREATE,
					Path: []string{"foo"},
					From: "foo",
					To:   "bar",
				},
				{
					Type: diff.CREATE,
					Path: []string{"foo"},
					From: "baz",
					To:   "bar",
				},
			},
			want: []rules.Rule{},
		},
		{
			name: "should handle nil to in safe",
			rules: []rules.Rule{
				{
					Path: ".foo",
					Reducers: &[]rules.Reducer{
						{
							From: "foo",
							To:   "bar",
						},
						{
							From: "baz",
							To:   "bar",
						},
					},
					Safe: &[]rules.Safe{
						{
							From: &foo,
						},
					},
				},
			},
			diffs: diff.Changelog{
				{
					Type: diff.CREATE,
					Path: []string{"foo"},
					From: "foo",
					To:   "bar",
				},
			},
			want: []rules.Rule{},
		},
		{
			name: "should handle nil reducers",
			rules: []rules.Rule{
				{
					Path:     ".foo",
					Reducers: nil,
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
					Type: diff.CREATE,
					Path: []string{"foo"},
					From: "foo",
					To:   "bar",
				},
			},
			want: []rules.Rule{},
		},
		{
			name: "should return rules if a diff matches and is not safe",
			rules: []rules.Rule{
				{
					Path: ".foo",
					Reducers: &[]rules.Reducer{
						{
							From: "foo",
							To:   "bar",
						},
					},
					Safe: &[]rules.Safe{
						{
							From: &foo,
							To:   &bar,
						},
					},
				},
				{
					Path: ".bar",
					Reducers: &[]rules.Reducer{
						{
							From: "bar",
							To:   "foo",
						},
					},
					Safe: &[]rules.Safe{
						{
							From: &bar,
							To:   &foo,
						},
					},
				},
				{
					Path: ".baz",
					Reducers: &[]rules.Reducer{
						{
							From: "baz",
							To:   "foo",
						},
					},
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
					Type: diff.CREATE,
					Path: []string{"foo"},
					From: "foo",
					To:   "bar",
				},
				{
					Type: diff.CREATE,
					Path: []string{"bar"},
					From: "bar",
					To:   "foo",
				},
				{
					Type: diff.CREATE,
					Path: []string{"baz"},
					From: "baz",
					To:   "foo",
				},
			},
			want: []rules.Rule{
				{
					Path: ".baz",
					Reducers: &[]rules.Reducer{
						{
							From: "baz",
							To:   "foo",
						},
					},
					Safe: &[]rules.Safe{
						{
							From: &foo,
							To:   &bar,
						},
					},
				},
			},
		},
		{
			name: "should handle from-nodes condition - matching",
			rules: []rules.Rule{
				{
					Path: ".spec.distribution.modules.logging.loki.tsdbStartDate",
					Reducers: &[]rules.Reducer{
						{
							From: nil,
							To:   tsdbDate,
						},
					},
					Safe: &[]rules.Safe{
						{
							FromNodes: &[]rules.FromNode{
								{
									Path: stringPtr(".spec.distribution.modules.logging.type"),
									From: stringPtr("none"),
								},
							},
						},
					},
				},
			},
			diffs: diff.Changelog{
				{
					Type: diff.UPDATE,
					Path: []string{"spec", "distribution", "modules", "logging", "type"},
					From: none,
					To:   loki,
				},
				{
					Type: diff.UPDATE,
					Path: []string{"spec", "distribution", "modules", "logging", "loki", "tsdbStartDate"},
					From: nil,
					To:   tsdbDate,
				},
			},
			want: []rules.Rule{}, // Empty because the rule is safe
		},
		{
			name: "should handle from-nodes condition - not matching",
			rules: []rules.Rule{
				{
					Path: ".spec.distribution.modules.logging.loki.tsdbStartDate",
					Reducers: &[]rules.Reducer{
						{
							From: nil,
							To:   tsdbDate,
						},
					},
					Safe: &[]rules.Safe{
						{
							FromNodes: &[]rules.FromNode{
								{
									Path: stringPtr(".spec.distribution.modules.logging.type"),
									From: stringPtr("none"),
								},
							},
						},
					},
				},
			},
			diffs: diff.Changelog{
				{
					Type: diff.UPDATE,
					Path: []string{"spec", "distribution", "modules", "logging", "type"},
					From: loki, // Not "none", so the condition doesn't match
					To:   loki,
				},
				{
					Type: diff.UPDATE,
					Path: []string{"spec", "distribution", "modules", "logging", "loki", "tsdbStartDate"},
					From: nil,
					To:   tsdbDate,
				},
			},
			want: []rules.Rule{
				// The rule is returned because it's not safe
				{
					Path: ".spec.distribution.modules.logging.loki.tsdbStartDate",
					Reducers: &[]rules.Reducer{
						{
							From: nil,
							To:   tsdbDate,
						},
					},
					Safe: &[]rules.Safe{
						{
							FromNodes: &[]rules.FromNode{
								{
									Path: stringPtr(".spec.distribution.modules.logging.type"),
									From: stringPtr("none"),
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			x := rules.NewBaseExtractor(rules.Spec{})

			got := x.UnsafeReducerRulesByDiffs(tc.rules, tc.diffs)

			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("expected %v, got %v", tc.want, got)
			}
		})
	}
}

func TestBaseExtractor_ExtractReducerRules(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		rules []rules.Rule
		want  []rules.Rule
	}{
		{
			name:  "should return empty slice if no rules",
			rules: []rules.Rule{},
			want:  []rules.Rule{},
		},
		{
			name: "should return empty slice if no reducers",
			rules: []rules.Rule{
				{
					Path: ".foo",
				},
			},
			want: []rules.Rule{},
		},
		{
			name: "should return reducers",
			rules: []rules.Rule{
				{
					Path: ".foo",
					Reducers: &[]rules.Reducer{
						{
							From: "foo",
							To:   "bar",
						},
					},
				},
				{
					Path: ".bar",
					Reducers: &[]rules.Reducer{
						{
							From: "bar",
							To:   "foo",
						},
					},
				},
			},
			want: []rules.Rule{
				{
					Path: ".foo",
					Reducers: &[]rules.Reducer{
						{
							From: "foo",
							To:   "bar",
						},
					},
				},
				{
					Path: ".bar",
					Reducers: &[]rules.Reducer{
						{
							From: "bar",
							To:   "foo",
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			x := rules.NewBaseExtractor(rules.Spec{})

			got := x.ExtractReducerRules(tc.rules)

			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("expected %v, got %v", tc.want, got)
			}
		})
	}
}

func TestBaseExtractor_FilterSafeImmutableRules(t *testing.T) {
	t.Parallel()

	var foo, bar, none, loki, tsdbDate any

	foo = "foo"
	bar = "bar"
	none = "none"
	loki = "loki"
	tsdbDate = "2023-01-01"

	stringPtr := func(s string) *string {
		return &s
	}

	anyPtr := func(a any) *any {
		return &a
	}

	testCases := []struct {
		name  string
		rules []rules.Rule
		diffs diff.Changelog
		want  []rules.Rule
	}{
		{
			name:  "should return empty slice if no rules",
			rules: []rules.Rule{},
			diffs: diff.Changelog{
				{
					Type: diff.CREATE,
					Path: []string{"foo"},
				},
			},
			want: []rules.Rule{},
		},
		{
			name: "should handle nil diffs",
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
			diffs: nil,
			want: []rules.Rule{
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
		},
		{
			name: "should handle nil safe",
			rules: []rules.Rule{
				{
					Path:      ".foo",
					Immutable: true,
					Safe:      nil,
				},
			},
			diffs: diff.Changelog{
				{
					Type: diff.CREATE,
					Path: []string{"foo"},
					From: "foo",
					To:   "bar",
				},
			},
			want: []rules.Rule{
				{
					Path:      ".foo",
					Immutable: true,
					Safe:      nil,
				},
			},
		},
		{
			name: "should handle empty safe",
			rules: []rules.Rule{
				{
					Path:      ".foo",
					Immutable: true,
					Safe:      &[]rules.Safe{},
				},
			},
			diffs: diff.Changelog{
				{
					Type: diff.CREATE,
					Path: []string{"foo"},
					From: "foo",
					To:   "bar",
				},
			},
			want: []rules.Rule{
				{
					Path:      ".foo",
					Immutable: true,
					Safe:      &[]rules.Safe{},
				},
			},
		},
		{
			name: "should filter out rules with matching from/to conditions",
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
				{
					Path:      ".bar",
					Immutable: true,
					Safe: &[]rules.Safe{
						{
							From: &bar,
							To:   &foo,
						},
					},
				},
			},
			diffs: diff.Changelog{
				{
					Type: diff.CREATE,
					Path: []string{"foo"},
					From: "foo",
					To:   "bar",
				},
				{
					Type: diff.CREATE,
					Path: []string{"bar"},
					From: "bar",
					To:   "foo",
				},
			},
			want: []rules.Rule{}, // Both rules are filtered out because they match their safe conditions
		},
		{
			name: "should keep rules without matching from/to conditions",
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
				{
					Path:      ".bar",
					Immutable: true,
					Safe: &[]rules.Safe{
						{
							From: &bar,
							To:   &bar, // Doesn't match the diff
						},
					},
				},
			},
			diffs: diff.Changelog{
				{
					Type: diff.CREATE,
					Path: []string{"foo"},
					From: "foo",
					To:   "bar",
				},
				{
					Type: diff.CREATE,
					Path: []string{"bar"},
					From: "bar",
					To:   "foo",
				},
			},
			want: []rules.Rule{
				{
					Path:      ".bar",
					Immutable: true,
					Safe: &[]rules.Safe{
						{
							From: &bar,
							To:   &bar,
						},
					},
				},
			},
		},
		{
			name: "should filter out rules with matching FromNodes conditions",
			rules: []rules.Rule{
				{
					Path:      ".spec.distribution.modules.logging.loki.tsdbStartDate",
					Immutable: true,
					Safe: &[]rules.Safe{
						{
							FromNodes: &[]rules.FromNode{
								{
									Path: stringPtr(".spec.distribution.modules.logging.type"),
									From: stringPtr("none"),
								},
							},
						},
					},
				},
			},
			diffs: diff.Changelog{
				{
					Type: diff.UPDATE,
					Path: []string{"spec", "distribution", "modules", "logging", "type"},
					From: none,
					To:   loki,
				},
				{
					Type: diff.UPDATE,
					Path: []string{"spec", "distribution", "modules", "logging", "loki", "tsdbStartDate"},
					From: nil,
					To:   tsdbDate,
				},
			},
			want: []rules.Rule{}, // Rule is filtered out because FromNodes condition matches
		},
		{
			name: "should keep rules without matching FromNodes conditions",
			rules: []rules.Rule{
				{
					Path:      ".spec.distribution.modules.logging.loki.tsdbStartDate",
					Immutable: true,
					Safe: &[]rules.Safe{
						{
							FromNodes: &[]rules.FromNode{
								{
									Path: stringPtr(".spec.distribution.modules.logging.type"),
									From: stringPtr("none"),
								},
							},
						},
					},
				},
			},
			diffs: diff.Changelog{
				{
					Type: diff.UPDATE,
					Path: []string{"spec", "distribution", "modules", "logging", "type"},
					From: loki, // Not "none", so the condition doesn't match
					To:   loki,
				},
				{
					Type: diff.UPDATE,
					Path: []string{"spec", "distribution", "modules", "logging", "loki", "tsdbStartDate"},
					From: nil,
					To:   tsdbDate,
				},
			},
			want: []rules.Rule{
				{
					Path:      ".spec.distribution.modules.logging.loki.tsdbStartDate",
					Immutable: true,
					Safe: &[]rules.Safe{
						{
							FromNodes: &[]rules.FromNode{
								{
									Path: stringPtr(".spec.distribution.modules.logging.type"),
									From: stringPtr("none"),
								},
							},
						},
					},
				},
			},
		},
		{
			name: "should filter out rules with matching combined conditions",
			rules: []rules.Rule{
				{
					Path:      ".spec.distribution.modules.logging.loki.tsdbStartDate",
					Immutable: true,
					Safe: &[]rules.Safe{
						{
							From: anyPtr(nil),
							To:   anyPtr(tsdbDate),
							FromNodes: &[]rules.FromNode{
								{
									Path: stringPtr(".spec.distribution.modules.logging.type"),
									From: stringPtr("none"),
								},
							},
						},
					},
				},
			},
			diffs: diff.Changelog{
				{
					Type: diff.UPDATE,
					Path: []string{"spec", "distribution", "modules", "logging", "type"},
					From: none,
					To:   loki,
				},
				{
					Type: diff.UPDATE,
					Path: []string{"spec", "distribution", "modules", "logging", "loki", "tsdbStartDate"},
					From: nil,
					To:   tsdbDate,
				},
			},
			want: []rules.Rule{}, // Rule is filtered out because both From/To and FromNodes conditions match
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			x := rules.NewBaseExtractor(rules.Spec{})

			got := x.FilterSafeImmutableRules(tc.rules, tc.diffs)

			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("expected %v, got %v", tc.want, got)
			}
		})
	}
}
