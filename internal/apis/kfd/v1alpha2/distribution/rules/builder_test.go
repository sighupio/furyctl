// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package rules_test

import (
	"reflect"
	"testing"

	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/distribution/rules"
)

func TestEKSBuilder_GetImmutables(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc            string
		distroRulesSpec *rules.DistroRulesSpec
		phase           string
		want            []string
	}{
		{
			desc:            "distribution - empty",
			distroRulesSpec: &rules.DistroRulesSpec{},
			phase:           "distribution",
			want:            nil,
		},
		{
			desc: "distribution - not empty",
			distroRulesSpec: &rules.DistroRulesSpec{
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

			builder := rules.DistroBuilder{
				Spec: *tC.distroRulesSpec,
			}

			got := builder.GetImmutables(tC.phase)

			if !reflect.DeepEqual(got, tC.want) {
				t.Errorf("expected immutables to be %v, got: %v", tC.want, got)
			}
		})
	}
}
