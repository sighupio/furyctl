// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package slices_test

import (
	"testing"

	"github.com/sighupio/furyctl/internal/x/slices"
)

func TestUniq(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "empty",
			in:   []string{},
			want: []string{},
		},
		{
			name: "one element",
			in:   []string{"a"},
			want: []string{"a"},
		},
		{
			name: "two elements",
			in:   []string{"a", "b"},
			want: []string{"a", "b"},
		},
		{
			name: "two elements, one duplicated",
			in:   []string{"a", "a"},
			want: []string{"a"},
		},
		{
			name: "three elements, one duplicated",
			in:   []string{"a", "b", "a"},
			want: []string{"a", "b"},
		},
		{
			name: "three elements, two duplicated",
			in:   []string{"a", "b", "b"},
			want: []string{"a", "b"},
		},
		{
			name: "three elements, all duplicated",
			in:   []string{"a", "a", "a"},
			want: []string{"a"},
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := slices.Uniq(tt.in)

			if len(got) != len(tt.want) {
				t.Errorf("got %d elements, want %d", len(got), len(tt.want))
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("got %s, want %s", got[i], tt.want[i])
				}
			}
		})
	}
}
