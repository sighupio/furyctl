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

func TestIntersection(t *testing.T) {
	t.Parallel()

	testCases := []struct {
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
			name: "one element disjoint",
			a:    []string{"a"},
			b:    []string{"b"},
			want: []string{},
		},
		{
			name: "one element not disjoint",
			a:    []string{"a"},
			b:    []string{"a"},
			want: []string{"a"},
		},
		{
			name: "one element replicated not disjoint",
			a:    []string{"a"},
			b:    []string{"a", "a"},
			want: []string{"a"},
		},
		{
			name: "not disjoint",
			a:    []string{"a", "b", "c"},
			b:    []string{"a", "d", "e"},
			want: []string{"a"},
		},
		{
			name: "not disjoint with multiple element duplicated and different length",
			a:    []string{"a", "b", "c", "a"},
			b:    []string{"a", "d", "e", "e", "a"},
			want: []string{"a"},
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := slices.Intersection(tc.a, tc.b)

			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Intersection() = %v, want %v", got, tc.want)
			}
		})
	}
}
