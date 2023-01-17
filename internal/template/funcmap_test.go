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

	"github.com/sighupio/furyctl/internal/template"
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
