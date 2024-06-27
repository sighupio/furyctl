// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mapx_test

import (
	"reflect"
	"testing"

	mapx "github.com/sighupio/furyctl/internal/x/map"
)

func TestFromStruct(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc      string
		sIn       any
		tag       string
		skipEmpty bool
		want      map[any]any
	}{
		{
			"input is nil",
			nil,
			"",
			false,
			nil,
		},
		{
			"input is not a struct",
			"not a struct",
			"",
			false,
			nil,
		},
		{
			"input is a struct with no tags",
			struct {
				A string
			}{
				A: "a",
			},
			"json",
			false,
			map[any]any{"A": "a"},
		},
		{
			"input is a struct with correct tags",
			struct {
				A string `json:"a"`
			}{
				A: "a",
			},
			"json",
			false,
			map[any]any{"a": "a"},
		},
		{
			"input is a struct with default values",
			struct {
				A string `json:"a"`
				B int    `json:"b"`
				C struct {
					D string `json:"d"`
				} `json:"c"`
				E *struct {
					F string `json:"f"`
				} `json:"e"`
			}{
				A: "a",
				B: 0,
				C: struct {
					D string `json:"d"`
				}{
					D: "",
				},
				E: nil,
			},
			"json",
			true,
			map[any]any{
				"a": "a",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()

			builder := mapx.NewBuilder(tc.skipEmpty)

			got := builder.FromStruct(tc.sIn, tc.tag)

			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("expected %v, got %v", tc.want, got)
			}
		})
	}
}

func TestToMapStringAny(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc string
		sIn  map[any]any
		want map[string]map[any]any
	}{
		{
			"input is nil",
			nil,
			map[string]map[any]any{},
		},
		{
			"input is not a map[any]map[any]any",
			map[any]any{
				"key": "value",
			},
			map[string]map[any]any{},
		},
		{
			"input is a map[any]map[any]any",
			map[any]any{
				"key": map[any]any{
					"key": "value",
				},
				"notMap": "notMap",
			},
			map[string]map[any]any{
				"key": {
					"key": "value",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()

			builder := mapx.NewBuilder(false)

			got := builder.ToMapStringAny(tc.sIn)

			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("expected %v, got %v", tc.want, got)
			}
		})
	}
}
