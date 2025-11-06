// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package template_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"

	"github.com/sighupio/furyctl/pkg/template"
)

func TestNewFuncMap(t *testing.T) {
	f := template.NewFuncMap()

	assert.True(t, len(f.FuncMap) > 0)
}

func TestFuncMap_Add(t *testing.T) {
	f := template.NewFuncMap()

	f.Add("test", func() string {
		return "test"
	})

	assert.NotNil(t, f.FuncMap["test"])
}

func TestFuncMap_Delete(t *testing.T) {
	f := template.NewFuncMap()

	f.Add("test", func() string {
		return "test"
	})

	f.Delete("test")

	assert.Nil(t, f.FuncMap["test"])
}

func TestToYAML(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc string
		data any
		want string
	}{
		{
			desc: "empty yaml",
			want: "null",
		},
		{
			desc: "simple yaml",
			data: map[string]string{
				"foo": "bar",
				"baz": "quux",
			},
			want: "baz: quux\nfoo: bar",
		},
		{
			desc: "broken yaml",
			data: map[string]func(){
				"foo": func() {},
			},
			want: "",
		},
	}
	for _, tC := range testCases {
		tC := tC

		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			got := template.ToYAML(tC.data)

			if got != tC.want {
				t.Fatalf("expected %q, got %q", tC.want, got)
			}
		})
	}
}

func TestFromYAML(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc string
		data string
		want map[string]any
	}{
		{
			desc: "empty yaml",
			data: "",
			want: map[string]any{},
		},
		{
			desc: "simple yaml",
			data: "baz: quux\nfoo: bar",
			want: map[string]any{
				"foo": "bar",
				"baz": "quux",
			},
		},
		{
			desc: "broken yaml",
			data: "baz:\n: quux\nfoo: bar",
			want: map[string]any{
				"Error": "yaml: line 1: did not find expected key",
			},
		},
	}
	for _, tC := range testCases {
		tC := tC

		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			got := template.FromYAML(tC.data)

			if !cmp.Equal(got, tC.want, cmpopts.EquateEmpty()) {
				t.Fatalf("expected %+v, got %+v", tC.want, got)
			}
		})
	}
}

func TestHasField(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc string
		data any
		path string
		want bool
	}{
		{
			desc: "nil data returns false",
			data: nil,
			path: "spec.kubernetes",
			want: false,
		},
		{
			desc: "empty path returns false",
			data: map[any]any{"spec": map[any]any{"kubernetes": "v1.28"}},
			path: "",
			want: false,
		},
		{
			desc: "single level field exists",
			data: map[any]any{"name": "test"},
			path: "name",
			want: true,
		},
		{
			desc: "single level field missing",
			data: map[any]any{"name": "test"},
			path: "other",
			want: false,
		},
		{
			desc: "nested field exists - map[any]any",
			data: map[any]any{
				"spec": map[any]any{
					"kubernetes": map[any]any{
						"version": "1.28",
					},
				},
			},
			path: "spec.kubernetes.version",
			want: true,
		},
		{
			desc: "nested field missing at first level",
			data: map[any]any{
				"spec": map[any]any{
					"kubernetes": map[any]any{
						"version": "1.28",
					},
				},
			},
			path: "metadata.name",
			want: false,
		},
		{
			desc: "nested field missing at intermediate level",
			data: map[any]any{
				"spec": map[any]any{
					"kubernetes": map[any]any{
						"version": "1.28",
					},
				},
			},
			path: "spec.networking.version",
			want: false,
		},
		{
			desc: "nested field missing at leaf level",
			data: map[any]any{
				"spec": map[any]any{
					"kubernetes": map[any]any{
						"version": "1.28",
					},
				},
			},
			path: "spec.kubernetes.apiServer",
			want: false,
		},
		{
			desc: "field exists with nil value",
			data: map[any]any{
				"spec": map[any]any{
					"kubernetes": nil,
				},
			},
			path: "spec.kubernetes",
			want: true,
		},
		{
			desc: "field exists with empty map value",
			data: map[any]any{
				"spec": map[any]any{
					"kubernetes": map[any]any{},
				},
			},
			path: "spec.kubernetes",
			want: true,
		},
		{
			desc: "map[string]any data structure",
			data: map[string]any{
				"metadata": map[string]any{
					"labels": map[string]any{
						"env": "production",
					},
				},
			},
			path: "metadata.labels.env",
			want: true,
		},
		{
			desc: "mixed map types",
			data: map[any]any{
				"spec": map[string]any{
					"kubernetes": map[any]any{
						"version": "1.28",
					},
				},
			},
			path: "spec.kubernetes.version",
			want: true,
		},
		{
			desc: "path with empty segment",
			data: map[any]any{"spec": map[any]any{"kubernetes": "v1.28"}},
			path: "spec..kubernetes",
			want: false,
		},
		{
			desc: "path ending with dot",
			data: map[any]any{"spec": map[any]any{"kubernetes": "v1.28"}},
			path: "spec.",
			want: false,
		},
		{
			desc: "path starting with dot",
			data: map[any]any{"spec": map[any]any{"kubernetes": "v1.28"}},
			path: ".spec.kubernetes",
			want: false,
		},
		{
			desc: "non-map intermediate value",
			data: map[any]any{
				"spec": "string-value",
			},
			path: "spec.kubernetes",
			want: false,
		},
		{
			desc: "deeply nested field exists",
			data: map[any]any{
				"a": map[any]any{
					"b": map[any]any{
						"c": map[any]any{
							"d": map[any]any{
								"e": "value",
							},
						},
					},
				},
			},
			path: "a.b.c.d.e",
			want: true,
		},
		{
			desc: "deeply nested field missing at deep level",
			data: map[any]any{
				"a": map[any]any{
					"b": map[any]any{
						"c": map[any]any{
							"d": map[any]any{
								"e": "value",
							},
						},
					},
				},
			},
			path: "a.b.c.d.f",
			want: false,
		},
		{
			desc: "field with boolean value",
			data: map[any]any{
				"spec": map[any]any{
					"enabled": false,
				},
			},
			path: "spec.enabled",
			want: true,
		},
		{
			desc: "field with number value",
			data: map[any]any{
				"spec": map[any]any{
					"replicas": 3,
				},
			},
			path: "spec.replicas",
			want: true,
		},
		{
			desc: "field with slice value",
			data: map[any]any{
				"spec": map[any]any{
					"items": []string{"a", "b", "c"},
				},
			},
			path: "spec.items",
			want: true,
		},
		{
			desc: "nil value at intermediate level",
			data: map[any]any{
				"spec": map[any]any{
					"kubernetes": nil,
				},
			},
			path: "spec.kubernetes.version",
			want: false,
		},
		{
			desc: "string value at intermediate level",
			data: map[any]any{
				"spec": map[any]any{
					"kubernetes": "v1.28",
				},
			},
			path: "spec.kubernetes.version",
			want: false,
		},
		{
			desc: "number value at intermediate level",
			data: map[any]any{
				"spec": map[any]any{
					"replicas": 3,
				},
			},
			path: "spec.replicas.count",
			want: false,
		},
		{
			desc: "boolean value at intermediate level",
			data: map[any]any{
				"spec": map[any]any{
					"enabled": true,
				},
			},
			path: "spec.enabled.value",
			want: false,
		},
		{
			desc: "slice value at intermediate level",
			data: map[any]any{
				"spec": map[any]any{
					"items": []string{"a", "b"},
				},
			},
			path: "spec.items.first",
			want: false,
		},
	}

	for _, tC := range testCases {
		tC := tC

		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			got := template.HasField(tC.data, tC.path)

			if got != tC.want {
				t.Fatalf("expected %v, got %v", tC.want, got)
			}
		})
	}
}

func TestGetFieldOrDefault(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc         string
		data         any
		path         string
		defaultValue any
		want         any
	}{
		{
			desc:         "nil data returns default",
			data:         nil,
			path:         "spec.kubernetes",
			defaultValue: "default",
			want:         "default",
		},
		{
			desc:         "empty path returns default",
			data:         map[any]any{"spec": "value"},
			path:         "",
			defaultValue: "default",
			want:         "default",
		},
		{
			desc:         "single level field exists",
			data:         map[any]any{"name": "test"},
			path:         "name",
			defaultValue: "default",
			want:         "test",
		},
		{
			desc:         "single level field missing",
			data:         map[any]any{"name": "test"},
			path:         "other",
			defaultValue: "default",
			want:         "default",
		},
		{
			desc: "nested field exists - map[any]any",
			data: map[any]any{
				"spec": map[any]any{
					"kubernetes": map[any]any{
						"version": "1.28",
					},
				},
			},
			path:         "spec.kubernetes.version",
			defaultValue: "1.27",
			want:         "1.28",
		},
		{
			desc: "nested field missing at first level",
			data: map[any]any{
				"spec": map[any]any{
					"kubernetes": map[any]any{
						"version": "1.28",
					},
				},
			},
			path:         "metadata.name",
			defaultValue: "default-name",
			want:         "default-name",
		},
		{
			desc: "nested field missing at intermediate level",
			data: map[any]any{
				"spec": map[any]any{
					"kubernetes": map[any]any{
						"version": "1.28",
					},
				},
			},
			path:         "spec.networking.version",
			defaultValue: "v1",
			want:         "v1",
		},
		{
			desc: "nested field missing at leaf level",
			data: map[any]any{
				"spec": map[any]any{
					"kubernetes": map[any]any{
						"version": "1.28",
					},
				},
			},
			path:         "spec.kubernetes.apiServer",
			defaultValue: "default-api",
			want:         "default-api",
		},
		{
			desc: "field exists with nil value",
			data: map[any]any{
				"spec": map[any]any{
					"kubernetes": nil,
				},
			},
			path:         "spec.kubernetes",
			defaultValue: "default",
			want:         "default",
		},
		{
			desc: "field exists with empty map value",
			data: map[any]any{
				"spec": map[any]any{
					"kubernetes": map[any]any{},
				},
			},
			path:         "spec.kubernetes",
			defaultValue: "default",
			want:         map[any]any{},
		},
		{
			desc: "field exists with empty string value",
			data: map[any]any{
				"spec": map[any]any{
					"name": "",
				},
			},
			path:         "spec.name",
			defaultValue: "default",
			want:         "",
		},
		{
			desc: "field exists with zero number value",
			data: map[any]any{
				"spec": map[any]any{
					"replicas": 0,
				},
			},
			path:         "spec.replicas",
			defaultValue: 5,
			want:         0,
		},
		{
			desc: "field exists with false boolean value",
			data: map[any]any{
				"spec": map[any]any{
					"enabled": false,
				},
			},
			path:         "spec.enabled",
			defaultValue: true,
			want:         false,
		},
		{
			desc: "map[string]any data structure",
			data: map[string]any{
				"metadata": map[string]any{
					"labels": map[string]any{
						"env": "production",
					},
				},
			},
			path:         "metadata.labels.env",
			defaultValue: "development",
			want:         "production",
		},
		{
			desc: "mixed map types",
			data: map[any]any{
				"spec": map[string]any{
					"kubernetes": map[any]any{
						"version": "1.28",
					},
				},
			},
			path:         "spec.kubernetes.version",
			defaultValue: "1.27",
			want:         "1.28",
		},
		{
			desc:         "path with empty segment",
			data:         map[any]any{"spec": map[any]any{"kubernetes": "v1.28"}},
			path:         "spec..kubernetes",
			defaultValue: "default",
			want:         "default",
		},
		{
			desc:         "path ending with dot",
			data:         map[any]any{"spec": map[any]any{"kubernetes": "v1.28"}},
			path:         "spec.",
			defaultValue: "default",
			want:         "default",
		},
		{
			desc:         "path starting with dot",
			data:         map[any]any{"spec": map[any]any{"kubernetes": "v1.28"}},
			path:         ".spec.kubernetes",
			defaultValue: "default",
			want:         "default",
		},
		{
			desc: "non-map intermediate value",
			data: map[any]any{
				"spec": "string-value",
			},
			path:         "spec.kubernetes",
			defaultValue: "default",
			want:         "default",
		},
		{
			desc: "deeply nested field exists",
			data: map[any]any{
				"a": map[any]any{
					"b": map[any]any{
						"c": map[any]any{
							"d": map[any]any{
								"e": "value",
							},
						},
					},
				},
			},
			path:         "a.b.c.d.e",
			defaultValue: "default",
			want:         "value",
		},
		{
			desc: "deeply nested field missing at deep level",
			data: map[any]any{
				"a": map[any]any{
					"b": map[any]any{
						"c": map[any]any{
							"d": map[any]any{
								"e": "value",
							},
						},
					},
				},
			},
			path:         "a.b.c.d.f",
			defaultValue: "default",
			want:         "default",
		},
		{
			desc: "default value is complex type",
			data: map[any]any{
				"spec": map[any]any{},
			},
			path:         "spec.missing",
			defaultValue: map[string]string{"key": "value"},
			want:         map[string]string{"key": "value"},
		},
		{
			desc: "nil value at intermediate level",
			data: map[any]any{
				"spec": map[any]any{
					"kubernetes": nil,
				},
			},
			path:         "spec.kubernetes.version",
			defaultValue: "1.27",
			want:         "1.27",
		},
		{
			desc: "string value at intermediate level",
			data: map[any]any{
				"spec": map[any]any{
					"kubernetes": "v1.28",
				},
			},
			path:         "spec.kubernetes.version",
			defaultValue: "default",
			want:         "default",
		},
		{
			desc: "number value at intermediate level",
			data: map[any]any{
				"spec": map[any]any{
					"replicas": 3,
				},
			},
			path:         "spec.replicas.count",
			defaultValue: 0,
			want:         0,
		},
		{
			desc: "boolean value at intermediate level",
			data: map[any]any{
				"spec": map[any]any{
					"enabled": true,
				},
			},
			path:         "spec.enabled.value",
			defaultValue: false,
			want:         false,
		},
		{
			desc: "slice value at intermediate level",
			data: map[any]any{
				"spec": map[any]any{
					"items": []string{"a", "b"},
				},
			},
			path:         "spec.items.first",
			defaultValue: "default",
			want:         "default",
		},
		{
			desc: "retrieve slice value",
			data: map[any]any{
				"spec": map[any]any{
					"items": []string{"a", "b", "c"},
				},
			},
			path:         "spec.items",
			defaultValue: []string{},
			want:         []string{"a", "b", "c"},
		},
		{
			desc: "retrieve map value",
			data: map[any]any{
				"spec": map[any]any{
					"config": map[string]string{
						"key1": "value1",
						"key2": "value2",
					},
				},
			},
			path:         "spec.config",
			defaultValue: map[string]string{},
			want:         map[string]string{"key1": "value1", "key2": "value2"},
		},
	}

	for _, tC := range testCases {
		tC := tC

		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			got := template.GetFieldOrDefault(tC.data, tC.path, tC.defaultValue)

			if !cmp.Equal(got, tC.want, cmpopts.EquateEmpty()) {
				t.Fatalf("expected %+v, got %+v", tC.want, got)
			}
		})
	}
}
