// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package rulesextractor_test

import (
	"reflect"
	"testing"

	"github.com/sighupio/furyctl/internal/rules"
	rules_extractor "github.com/sighupio/furyctl/pkg/rulesextractor"
)

func TestKFDBuilder_GetImmutables(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc  string
		Spec  *rules.Spec
		phase string
		want  []string
	}{
		{
			desc:  "distribution - empty",
			Spec:  &rules.Spec{},
			phase: "distribution",
			want:  []string{},
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
			want:  []string{"foo"},
		},
	}

	for _, tC := range testCases {
		tC := tC

		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			builder := rules_extractor.DistroExtractor{
				Spec: *tC.Spec,
			}

			got := builder.GetImmutables(tC.phase)

			if !reflect.DeepEqual(got, tC.want) {
				t.Errorf("expected immutables to be %v, got: %v", tC.want, got)
			}
		})
	}
}
