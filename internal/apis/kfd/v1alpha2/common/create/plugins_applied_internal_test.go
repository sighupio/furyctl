// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package create

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPluginsApplied(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		plugins map[any]any
		want    bool
	}{
		{name: "empty", plugins: map[any]any{}},
		{
			name:    "helm repositories only",
			plugins: map[any]any{"helm": map[any]any{"repositories": []any{map[any]any{"name": "longhorn"}}}},
		},
		{name: "empty helm releases", plugins: map[any]any{"helm": map[any]any{"releases": []any{}}}},
		{name: "empty kustomize", plugins: map[any]any{"kustomize": []any{}}},
		{
			name:    "helm release",
			plugins: map[any]any{"helm": map[any]any{"releases": []any{map[any]any{"name": "longhorn"}}}},
			want:    true,
		},
		{
			name:    "kustomize entry",
			plugins: map[any]any{"kustomize": []any{map[any]any{"name": "local-storage"}}},
			want:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, pluginsApplied(tc.plugins))
		})
	}
}
