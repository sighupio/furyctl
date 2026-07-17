// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package create

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// TestRawNodesByHostnamePassesThroughEveryField is the data-passthrough contract
// for the butane phase: the raw node handed to the templates must carry every
// sub-tree the templates read, including the free-form ones furyctl does not model
// on its typed config struct (network.ethernets, storage.{files,links,directories,
// additionalDisks}, systemd.units, passwd). If a future refactor reintroduces a
// lossy typed round-trip, these assertions fail.
func TestRawNodesByHostnamePassesThroughEveryField(t *testing.T) {
	t.Parallel()

	const sample = `
spec:
  infrastructure:
    nodes:
      - hostname: node01.example.com
        macAddress: 52:54:00:10:00:01
        arch: x86-64
        kernelParameters:
          - quiet
        storage:
          installDisk: /dev/sda
          additionalDisks:
            - device: /dev/sdb
          files:
            - path: /etc/example
          links:
            - path: /etc/link
          directories:
            - path: /etc/dir
        network:
          ethernets:
            eth0:
              addresses:
                - 192.168.1.10/24
        systemd:
          units:
            - name: example.service
        passwd:
          users:
            - name: core
`

	var conf map[any]any
	err := yaml.Unmarshal([]byte(sample), &conf)
	require.NoError(t, err, "unmarshal sample")

	nodes, err := rawNodesByHostname(conf)
	require.NoError(t, err, "rawNodesByHostname")

	raw, ok := nodes["node01.example.com"].(map[any]any)
	require.True(t, ok, "node not indexed by hostname, got %#v", nodes)

	for _, key := range []string{"kernelParameters", "network", "systemd", "passwd", "storage"} {
		assert.Contains(t, raw, key, "node sub-tree %q was not passed through", key)
	}

	network, ok := raw["network"].(map[any]any)
	require.True(t, ok, "network is not a map, got %T", raw["network"])

	assert.Contains(t, network, "ethernets", "network.ethernets was not passed through")

	storage, ok := raw["storage"].(map[any]any)
	require.True(t, ok, "storage is not a map, got %T", raw["storage"])

	for _, key := range []string{"installDisk", "additionalDisks", "files", "links", "directories"} {
		assert.Contains(t, storage, key, "storage.%s was not passed through", key)
	}
}

// TestRawNodesByHostnameArchDefault guards the arch backfill: .node.arch is read
// unguarded by the butane templates and the distribution schema defaults it to
// x86-64, but furyctl does not apply JSON-schema defaults before this phase. A node
// that omits arch (or leaves it blank) must come out as x86-64; an explicit arch
// must be left untouched.
func TestRawNodesByHostnameArchDefault(t *testing.T) {
	t.Parallel()

	const sample = `
spec:
  infrastructure:
    nodes:
      - hostname: omitted.example.com
        storage:
          installDisk: /dev/sda
      - hostname: blank.example.com
        arch: "  "
        storage:
          installDisk: /dev/sda
      - hostname: explicit.example.com
        arch: arm64
        storage:
          installDisk: /dev/sda
`

	var conf map[any]any
	err := yaml.Unmarshal([]byte(sample), &conf)
	require.NoError(t, err, "unmarshal sample")

	nodes, err := rawNodesByHostname(conf)
	require.NoError(t, err, "rawNodesByHostname")

	want := map[string]string{
		"omitted.example.com":  defaultNodeArch,
		"blank.example.com":    defaultNodeArch,
		"explicit.example.com": "arm64",
	}

	for hostname, wantArch := range want {
		node, ok := nodes[hostname].(map[any]any)
		require.True(t, ok, "%s not indexed", hostname)

		got, _ := node["arch"].(string)
		assert.Equal(t, wantArch, got, "%s: arch", hostname)
	}
}

func TestRawNodesByHostnameErrors(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"missing spec":           `kind: Immutable`,
		"missing infrastructure": "spec: {}",
		"nodes not a list":       "spec:\n  infrastructure:\n    nodes: nope",
		"node without hostname":  "spec:\n  infrastructure:\n    nodes:\n      - macAddress: x",
	}

	for name, sample := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var conf map[any]any
			err := yaml.Unmarshal([]byte(sample), &conf)
			require.NoError(t, err, "unmarshal sample")

			_, err = rawNodesByHostname(conf)
			assert.ErrorIs(t, err, ErrImmutableConfigMalformed)
		})
	}
}
