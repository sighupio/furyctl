// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slices_test

import (
	"reflect"
	"testing"

	"github.com/sighupio/furyctl/internal/x/slices"
)

func TestClean(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		s    []string
		want []string
	}{
		{
			name: "empty",
			s:    []string{},
			want: []string{},
		},
		{
			name: "one element",
			s:    []string{"a"},
			want: []string{"a"},
		},
		{
			name: "one zero",
			s:    []string{"a", "", "c"},
			want: []string{"a", "c"},
		},
		{
			name: "all zero",
			s:    []string{"", "", ""},
			want: []string{},
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := slices.Clean(tt.s); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Clean() = %v, want %v", got, tt.want)
			}
		})
	}
}
