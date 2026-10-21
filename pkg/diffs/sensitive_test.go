// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package diffs_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sighupio/furyctl/pkg/diffs"
)

func TestIsSensitivePath(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		path string
		want bool
	}{
		{"a key that contains pass", "spec.infrastructure.loadBalancers.keepalived.passphrase", true},
		{"a key that contains secret", "spec.distribution.modules.auth.dex.clientSecret", true},
		{"a key that contains token", "spec.plugins.helm.releases.0.set.apiToken", true},
		{"a key that contains key", "spec.distribution.modules.auth.oidcKubernetesAuth.sessionSecurityKey", true},
		{"the words are matched ignoring case", "spec.distribution.modules.auth.PASSWORD", true},
		{"a key in upper case that contains token", "spec.toolsConfiguration.TOKEN_FILE", true},
		{"a sensitive key above the leaf", "spec.distribution.modules.auth.dex.clientSecret.value", true},
		{"a sensitive key above a list position", "spec.kubernetes.ssh.keys.0", true},
		{"a path with none of the four words", "spec.distribution.modules.logging.type", false},
		{"a path with a list position and no word", "spec.kubernetes.nodeGroups.0.nodes.3.hostname", false},
		{"the path of the kind is not sensitive", "kind", false},
		{"an empty path", "", false},
	}

	for _, tC := range testCases {
		t.Run(tC.name, func(t *testing.T) {
			t.Parallel()

			var path []string
			if tC.path != "" {
				path = strings.Split(tC.path, ".")
			}

			assert.Equal(t, tC.want, diffs.IsSensitivePath(path))
		})
	}
}

func TestMaskSensitive(t *testing.T) {
	t.Parallel()

	t.Run("the value of a sensitive key is hidden at any depth", func(t *testing.T) {
		t.Parallel()

		value := []any{
			map[string]any{
				"id":           "ldap",
				"bindPassword": "sentinel-json-71d3",
				"config": map[string]any{
					"host":    "ldap.example.test",
					"API_KEY": "sentinel-deep-10aa",
					"oidc":    []any{map[string]any{"clientSecret": map[string]any{"value": "sentinel-deep-20bb"}}},
				},
			},
		}

		want := []any{
			map[string]any{
				"id":           "ldap",
				"bindPassword": "(hidden)",
				"config": map[string]any{
					"host":    "ldap.example.test",
					"API_KEY": "(hidden)",
					"oidc":    []any{map[string]any{"clientSecret": "(hidden)"}},
				},
			},
		}

		assert.Equal(t, want, diffs.MaskSensitive(nil, value))
	})

	t.Run("the value given is not changed", func(t *testing.T) {
		t.Parallel()

		inner := map[string]any{"token": "sentinel-copy-30cc", "name": "a"}
		value := map[string]any{"items": []any{inner}}

		masked := diffs.MaskSensitive(nil, value)

		assert.Equal(t, map[string]any{"items": []any{map[string]any{"token": "(hidden)", "name": "a"}}}, masked)
		assert.Equal(t, "sentinel-copy-30cc", inner["token"])
	})

	t.Run("an object with keys that are not strings", func(t *testing.T) {
		t.Parallel()

		value := map[any]any{"password": "sentinel-anykey-40dd", 1: "one"}

		assert.Equal(t, map[string]any{"password": "(hidden)", "1": "one"}, diffs.MaskSensitive(nil, value))
	})

	t.Run("a value without sensitive keys stays as it is", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "loki", diffs.MaskSensitive(nil, "loki"))
		assert.Equal(t, 3, diffs.MaskSensitive(nil, 3))
		assert.Nil(t, diffs.MaskSensitive(nil, nil))
		assert.Equal(t, map[string]any{}, diffs.MaskSensitive(nil, map[string]any{}))
		assert.Equal(
			t,
			[]any{map[string]any{"effect": "NoSchedule", "operator": "Exists"}},
			diffs.MaskSensitive(nil, []any{map[string]any{"effect": "NoSchedule", "operator": "Exists"}}),
		)
	})
}

// IsSensitivePath gives the same answer whatever IsListPosition says, because no
// sensitive word is made of digits. The order of the rows of the table depends on it,
// so its edges are checked here: the empty segment and the characters next to 0 and 9.
func TestIsListPosition(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		segment string
		want    bool
	}{
		{"the first digit", "0", true},
		{"the last digit", "9", true},
		{"a position of two digits", "10", true},
		{"an empty segment is not a position", "", false},
		{"the character before 0", "/", false},
		{"the character after 9", ":", false},
		{"a digit followed by a letter", "1a", false},
		{"a negative number", "-1", false},
		{"a key", "hostname", false},
	}

	for _, tC := range testCases {
		t.Run(tC.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tC.want, diffs.IsListPosition(tC.segment))
		})
	}
}
