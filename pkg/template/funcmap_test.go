// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package template_test

import (
	"errors"
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

func TestDigAny_Success(t *testing.T) {
	dict := map[any]any{
		"a": map[any]any{"b": "value"},
	}

	got, err := template.DigAny("a", "b", "default", dict)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "value" {
		t.Fatalf("expected %q, got %v", "value", got)
	}
}

func TestDigAny_MissingKeyReturnsDefault(t *testing.T) {
	dict := map[any]any{"a": map[any]any{"b": "value"}}

	got, err := template.DigAny("a", "x", "DEF", dict)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "DEF" {
		t.Fatalf("expected default %q, got %v", "DEF", got)
	}
}

func TestDigAny_InsufficientArgs(t *testing.T) {
	_, err := template.DigAny("only-one")
	if !errors.Is(err, template.ErrDigAnyInsufficientArgs) {
		t.Fatalf("expected ErrDigAnyInsufficientArgs, got %v", err)
	}
}

func TestDigAny_NonStringKey(t *testing.T) {
	dict := map[any]any{"a": map[any]any{"b": "value"}}
	_, err := template.DigAny(123, "default", dict)
	if err == nil || !errors.Is(err, template.ErrDigAnyInvalidKeyType) {
		t.Fatalf("expected ErrDigAnyInvalidKeyType, got %v", err)
	}
}

func TestDigAny_LastArgNotMap(t *testing.T) {
	_, err := template.DigAny("a", "default", 123)
	if err == nil || !errors.Is(err, template.ErrDigAnyInvalidDictType) {
		t.Fatalf("expected ErrDigAnyInvalidDictType, got %v", err)
	}
}

func TestDigAny_NestedNotMapReturnsDefault(t *testing.T) {
	dict := map[any]any{"a": "not-a-map"}
	got, err := template.DigAny("a", "b", "DEF", dict)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "DEF" {
		t.Fatalf("expected default %q, got %v", "DEF", got)
	}
}
