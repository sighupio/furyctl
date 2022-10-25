// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package template_test

import (
	"reflect"
	"testing"

	"github.com/sighupio/furyctl/internal/merge"
	"github.com/sighupio/furyctl/internal/template"
)

func TestNewConfig(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc      string
		tplSource *merge.Merger
		data      *merge.Merger
		excluded  []string
		want      template.Config
		wantErr   bool
		err       error
	}{
		{
			desc:      "should return an error if custom in merger is nil",
			tplSource: &merge.Merger{},
			data:      &merge.Merger{},
			excluded:  []string{},
			want:      template.Config{},
			wantErr:   true,
			err:       template.ErrTemplateSourceCustomIsNil,
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
			want:     template.Config{},
			wantErr:  true,
			err:      template.ErrDataSourceBaseIsNil,
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
			want: template.Config{
				Templates: template.Templates{
					Includes: []string{},
					Excludes: []string{"foo", "bar", "baz", "ar"},
				},
				Include: nil,
				Data: map[string]map[any]any{
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
		tc := tc
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()

			got, err := template.NewConfig(tc.tplSource, tc.data, tc.excluded)
			if err != nil {
				if !tc.wantErr {
					t.Fatalf("unexpected error: %v", err)
				}
				if !reflect.DeepEqual(err, tc.err) {
					t.Fatalf("want error %v, got %v", tc.err, err)
				}
			}

			if err == nil && tc.wantErr {
				t.Fatalf("expected error but got nil")
			}

			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("NewConfig() got = %v, want %v", got, tc.want)
			}
		})
	}
}
