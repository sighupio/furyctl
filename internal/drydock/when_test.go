// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package drydock

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The same table lives in ui/_tests/when.test.js. Keep them identical.
func TestWhenEval(t *testing.T) {
	t.Parallel()

	root := map[string]any{
		"topology": map[string]any{"lbMode": "dedicated", "lbCount": float64(2), "dedicatedEtcd": false},
		"modules":  map[string]any{"networking": map[string]any{"type": "cilium"}},
	}

	scope, ok := root["topology"].(map[string]any)
	require.True(t, ok)

	cases := []struct {
		expr string
		want bool
	}{
		{"lbMode == dedicated", true},
		{"lbMode != dedicated", false},
		{"lbCount == 2", true},
		{"lbCount == 1", false},
		{"dedicatedEtcd", false},
		{"!dedicatedEtcd", true},
		{"lbMode == dedicated && lbCount == 2", true},
		{"lbMode == dedicated && lbCount == 1", false},
		{"lbMode == none || lbCount == 2", true},
		{"lbMode == none || lbCount == 1", false},
		{"lbMode == none || lbMode == dedicated && lbCount == 2", true},
		{"topology.lbMode == dedicated", true},
		{"modules.networking.type == cilium", true},
		{"missing", false},
		{"!missing", true},
		{"missing == x", false},
		{"missing != x", true},
	}

	for _, tc := range cases {
		cond, err := ParseWhen(tc.expr)
		require.NoError(t, err, tc.expr)
		assert.Equal(t, tc.want, cond.Eval(scope, root), tc.expr)
	}
}

func TestParseWhenRejectsGarbage(t *testing.T) {
	t.Parallel()

	for _, expr := range []string{"a ==", "== a", "a == b c", "a && ", "(a)", "a > 1"} {
		_, err := ParseWhen(expr)
		require.ErrorIs(t, err, ErrInvalidWhen, expr)
	}
}

func TestEmptyWhenIsAlwaysTrue(t *testing.T) {
	t.Parallel()

	cond, err := ParseWhen("")
	require.NoError(t, err)
	assert.True(t, cond.Eval(nil, nil))
}
