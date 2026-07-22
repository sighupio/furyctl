// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package templatex_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/pkg/merge"
	templatex "github.com/sighupio/furyctl/pkg/template"
)

func TestNewConfig(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc      string
		tplSource *merge.Merger
		data      *merge.Merger
		excluded  []string
		want      templatex.Config
		wantErr   bool
		err       error
	}{
		{
			desc:      "should return an error if custom in merger is nil",
			tplSource: &merge.Merger{},
			data:      &merge.Merger{},
			excluded:  []string{},
			want:      templatex.Config{},
			wantErr:   true,
			err:       templatex.ErrTemplateSourceCustomIsNil,
		},
		{
			desc: "should return an error if data source base is nil",
			tplSource: merge.NewMerger(
				merge.NewDefaultModel(map[any]any{
					"data": map[any]any{
						"test": map[any]any{
							"foo": "bar",
						},
					},
				}, ".data"),
				merge.NewDefaultModel(map[any]any{
					"templates": map[any]any{
						"template": map[any]any{
							"foo": "bar",
						},
					},
				}, ".data"),
			),
			data:     &merge.Merger{},
			excluded: []string{},
			want:     templatex.Config{},
			wantErr:  true,
			err:      templatex.ErrDataSourceBaseIsNil,
		},
		{
			desc: "should return a config with the correct values",
			tplSource: merge.NewMerger(
				merge.NewDefaultModel(map[any]any{
					"data": map[any]any{
						"test": map[any]any{
							"foo": "bar",
						},
					},
				}, ".data"),
				merge.NewDefaultModel(map[any]any{
					"templates": map[any]any{
						"excludes": []any{
							"foo", "bar",
						},
					},
					"data": map[any]any{
						"test": map[any]any{
							"foo2": "bar2",
						},
					},
				}, ".data"),
			),
			data: merge.NewMerger(
				merge.NewDefaultModel(map[any]any{
					"parentTest": map[any]any{
						"test": map[any]any{
							"foo": "bar",
						},
					},
				}, ".data"),
				merge.NewDefaultModel(map[any]any{
					"parentTest": map[any]any{
						"test": map[any]any{
							"foo2": "bar2",
						},
					},
				}, ".data"),
			),
			excluded: []string{"baz", "ar"},
			want: templatex.Config{
				Templates: templatex.Templates{
					Includes: []string{},
					Excludes: []string{"foo", "bar", "baz", "ar"},
				},
				Include: nil,
				Data: map[string]map[any]any{
					"options": {
						"dryRun": false,
					},
					"parentTest": {
						"test": map[any]any{
							"foo": "bar",
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()

			got, err := templatex.NewConfig(tc.tplSource, tc.data, tc.excluded)
			if err != nil {
				if !tc.wantErr {
					t.Fatalf("unexpected error: %v", err)
				}

				if !errors.Is(err, tc.err) {
					t.Fatalf("want error %v, got %v", tc.err, err)
				}
			}

			if err == nil && tc.wantErr {
				t.Fatalf("expected error but got nil")
			}

			require.Equal(t, tc.want, got)
		})
	}
}
