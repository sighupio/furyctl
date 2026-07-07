// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package create

import (
	"errors"
	"testing"

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
	if err := yaml.Unmarshal([]byte(sample), &conf); err != nil {
		t.Fatalf("unmarshal sample: %v", err)
	}

	nodes, err := rawNodesByHostname(conf)
	if err != nil {
		t.Fatalf("rawNodesByHostname: %v", err)
	}

	raw, ok := nodes["node01.example.com"].(map[any]any)
	if !ok {
		t.Fatalf("node not indexed by hostname, got %#v", nodes)
	}

	for _, key := range []string{"kernelParameters", "network", "systemd", "passwd", "storage"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("node sub-tree %q was not passed through", key)
		}
	}

	network, ok := raw["network"].(map[any]any)
	if !ok {
		t.Fatalf("network is not a map, got %T", raw["network"])
	}

	if _, ok := network["ethernets"]; !ok {
		t.Errorf("network.ethernets was not passed through")
	}

	storage, ok := raw["storage"].(map[any]any)
	if !ok {
		t.Fatalf("storage is not a map, got %T", raw["storage"])
	}

	for _, key := range []string{"installDisk", "additionalDisks", "files", "links", "directories"} {
		if _, ok := storage[key]; !ok {
			t.Errorf("storage.%s was not passed through", key)
		}
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
	if err := yaml.Unmarshal([]byte(sample), &conf); err != nil {
		t.Fatalf("unmarshal sample: %v", err)
	}

	nodes, err := rawNodesByHostname(conf)
	if err != nil {
		t.Fatalf("rawNodesByHostname: %v", err)
	}

	want := map[string]string{
		"omitted.example.com":  defaultNodeArch,
		"blank.example.com":    defaultNodeArch,
		"explicit.example.com": "arm64",
	}

	for hostname, wantArch := range want {
		node, ok := nodes[hostname].(map[any]any)
		if !ok {
			t.Fatalf("%s not indexed", hostname)
		}

		if got, _ := node["arch"].(string); got != wantArch {
			t.Errorf("%s: arch = %q, want %q", hostname, got, wantArch)
		}
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
			if err := yaml.Unmarshal([]byte(sample), &conf); err != nil {
				t.Fatalf("unmarshal sample: %v", err)
			}

			if _, err := rawNodesByHostname(conf); !errors.Is(err, ErrImmutableConfigMalformed) {
				t.Errorf("expected ErrImmutableConfigMalformed, got %v", err)
			}
		})
	}
}
