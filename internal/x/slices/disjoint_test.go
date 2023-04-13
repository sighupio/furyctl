// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slices_test

import (
	"strings"
	"testing"

	"github.com/sighupio/furyctl/internal/x/slices"
)

func TestDisjoint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a    []string
		b    []string
		want bool
	}{
		{
			name: "empty",
			a:    []string{},
			b:    []string{},
			want: true,
		},
		{
			name: "one element",
			a:    []string{"a"},
			b:    []string{"b"},
			want: true,
		},
		{
			name: "not disjoint",
			a:    []string{"a", "b", "c"},
			b:    []string{"a", "d", "e"},
			want: false,
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := slices.Disjoint(tt.a, tt.b); got != tt.want {
				t.Errorf("Disjoint() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDisjointTransform(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		a          []string
		b          []string
		transformA slices.TransformFunc[string]
		transformB slices.TransformFunc[string]
		want       bool
	}{
		{
			name: "empty",
			a:    []string{},
			b:    []string{},
			want: true,
		},
		{
			name: "one element",
			a:    []string{"a"},
			b:    []string{"b"},
			want: true,
		},
		{
			name: "not disjoint",
			a:    []string{"a", "b", "c"},
			b:    []string{"a", "d", "e"},
			want: false,
		},
		{
			name:       "transform",
			a:          []string{"A", "B", "C"},
			b:          []string{"a", "b", "c"},
			transformA: strings.ToLower,
			transformB: strings.ToUpper,
			want:       true,
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := slices.DisjointTransform(tt.a, tt.b, tt.transformA, tt.transformB); got != tt.want {
				t.Errorf("DisjointTransform() = %v, want %v", got, tt.want)
			}
		})
	}
}
