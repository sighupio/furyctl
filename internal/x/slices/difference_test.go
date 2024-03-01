// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package slices_test

import (
	"reflect"
	"testing"

	"github.com/sighupio/furyctl/internal/x/slices"
)

func TestDifference(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a    []string
		b    []string
		want []string
	}{
		{
			name: "empty",
			a:    []string{},
			b:    []string{},
			want: []string{},
		},
		{
			name: "one element - not equal",
			a:    []string{"a"},
			b:    []string{"b"},
			want: []string{"a"},
		},
		{
			name: "one element - equal",
			a:    []string{"a"},
			b:    []string{"a"},
			want: []string{},
		},
		{
			name: "not disjoint",
			a:    []string{"a", "b", "c"},
			b:    []string{"a", "d", "e"},
			want: []string{"b", "c"},
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := slices.Difference(tt.a, tt.b); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Difference() = %v, want %v", got, tt.want)
			}
		})
	}
}
