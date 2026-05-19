// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package slicesx_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	slicesx "github.com/sighupio/furyctl/internal/x/slices"
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
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, slicesx.Clean(tt.s))
		})
	}
}
