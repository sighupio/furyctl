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

func TestMap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		a       []string
		mapFunc func(string) string
		want    []string
	}{
		{
			name: "empty",
			a:    []string{},
			mapFunc: func(s string) string {
				return s
			},
			want: []string{},
		},
		{
			name: "one element - identity function",
			a:    []string{"a"},
			mapFunc: func(s string) string {
				return s
			},
			want: []string{"a"},
		},
		{
			name: "n elements - add exclamation mark",
			a:    []string{"a", "b", "c"},
			mapFunc: func(s string) string {
				return s + "!"
			},
			want: []string{"a!", "b!", "c!"},
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := slices.Map(tt.a, tt.mapFunc); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Difference() = %v, want %v", got, tt.want)
			}
		})
	}
}
