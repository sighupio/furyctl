// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package rules_test

import (
	"reflect"
	"testing"

	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/onpremises/rules"
)

func TestEKSBuilder_GetImmutables(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc            string
		onPremRulesSpec *rules.OnPremRulesSpec
		phase           string
		want            []string
	}{
		{
			desc:            "kubernetes - empty",
			onPremRulesSpec: &rules.OnPremRulesSpec{},
			phase:           "kubernetes",
			want:            nil,
		},
		{
			desc:            "distribution - empty",
			onPremRulesSpec: &rules.OnPremRulesSpec{},
			phase:           "distribution",
			want:            nil,
		},
		{
			desc: "kubernetes - not empty",
			onPremRulesSpec: &rules.OnPremRulesSpec{
				Kubernetes: []rules.Rule{
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
			onPremRulesSpec: &rules.OnPremRulesSpec{
				Distribution: []rules.Rule{
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

			builder := rules.OnPremExtractor{
				Spec: *tC.onPremRulesSpec,
			}

			got := builder.GetImmutables(tC.phase)

			if !reflect.DeepEqual(got, tC.want) {
				t.Errorf("expected immutables to be %v, got: %v", tC.want, got)
			}
		})
	}
}
