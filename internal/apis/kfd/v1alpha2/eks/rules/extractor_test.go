// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package rules_test

import (
	"reflect"
	"testing"

	eksrules "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/eks/rules"
	"github.com/sighupio/furyctl/internal/rules"
)

func TestEKSBuilder_GetImmutables(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc         string
		eksRulesSpec *rules.Spec
		phase        string
		want         []string
	}{
		{
			desc:         "infrastructure - empty",
			eksRulesSpec: &rules.Spec{},
			phase:        "infrastructure",
			want:         []string{},
		},
		{
			desc:         "kubernetes - empty",
			eksRulesSpec: &rules.Spec{},
			phase:        "kubernetes",
			want:         []string{},
		},
		{
			desc:         "distribution - empty",
			eksRulesSpec: &rules.Spec{},
			phase:        "distribution",
			want:         []string{},
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
			want:  []string{"foo"},
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
			want:  []string{"foo"},
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
			want:  []string{"foo"},
		},
	}

	for _, tC := range testCases {
		tC := tC

		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			builder := eksrules.EKSExtractor{
				Spec: *tC.eksRulesSpec,
			}

			got := builder.GetImmutables(tC.phase)

			if !reflect.DeepEqual(got, tC.want) {
				t.Errorf("expected immutables to be %v, got: %v", tC.want, got)
			}
		})
	}
}
