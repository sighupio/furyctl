// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package upgradepath_test

import (
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/configs"
	"github.com/sighupio/furyctl/internal/upgradepath"
)

// fsWith builds an upgrade-paths filesystem holding one directory per given hop,
// mirroring the layout furyctl ships: upgrades/<kind>/<from>-<to>.
func fsWith(kind string, hops ...string) fstest.MapFS {
	fsys := fstest.MapFS{}

	for _, hop := range hops {
		fsys["upgrades/"+kind+"/"+hop+"/pre-distribution.sh.tpl"] = &fstest.MapFile{Data: []byte("#!/bin/sh\n")}
	}

	return fsys
}

// chainVersions flattens a chain into the versions it visits, starting from the source.
func chainVersions(chain []upgradepath.Hop) []string {
	if len(chain) == 0 {
		return nil
	}

	out := []string{chain[0].From}
	for _, hop := range chain {
		out = append(out, hop.To)
	}

	return out
}

func TestNext(t *testing.T) {
	t.Parallel()

	fsys := fsWith("onpremises", "1.32.0-1.32.1", "1.32.0-1.33.1", "1.32.0-1.33.0", "1.33.1-1.34.1")

	tests := []struct {
		name string
		kind string
		from string
		want []string
	}{
		{
			name: "targets are sorted lowest first and carry the v prefix",
			kind: "OnPremises",
			from: "1.32.0",
			want: []string{"v1.32.1", "v1.33.0", "v1.33.1"},
		},
		{
			name: "the v prefix on the input is accepted",
			kind: "OnPremises",
			from: "v1.32.0",
			want: []string{"v1.32.1", "v1.33.0", "v1.33.1"},
		},
		{
			name: "a version with no outgoing hop is a dead end",
			kind: "OnPremises",
			from: "1.34.1",
			want: nil,
		},
		{
			name: "an unknown kind yields nothing",
			kind: "NotAKind",
			from: "1.32.0",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, upgradepath.Next(fsys, tt.kind, tt.from), "Next")
		})
	}
}

func TestResolveChain(t *testing.T) {
	t.Parallel()

	// A graph shaped like the real one: a dead-end patch, a branch, and a long path.
	fsys := fsWith(
		"onpremises",
		"1.32.0-1.32.1",
		"1.32.0-1.33.0",
		"1.32.0-1.33.1",
		"1.33.0-1.33.1",
		"1.33.1-1.34.0",
		"1.33.1-1.34.1",
		"1.34.0-1.34.1",
		"1.34.1-1.35.0",
		"1.34.1-1.35.1",
		"1.35.0-1.35.1",
	)

	tests := []struct {
		name string
		from string
		to   string
		want []string
	}{
		{
			name: "already at the target is an empty chain, not an error",
			from: "1.35.1",
			to:   "1.35.1",
			want: nil,
		},
		{
			name: "a single supported hop",
			from: "1.34.1",
			to:   "1.35.1",
			want: []string{"v1.34.1", "v1.35.1"},
		},
		{
			name: "a dead-end patch forces an extra hop",
			from: "1.34.0",
			to:   "1.35.1",
			want: []string{"v1.34.0", "v1.34.1", "v1.35.1"},
		},
		{
			name: "three hops across three minors, through the highest intermediates",
			from: "1.32.0",
			to:   "1.35.1",
			want: []string{"v1.32.0", "v1.33.1", "v1.34.1", "v1.35.1"},
		},
		{
			name: "the v prefix is accepted on both ends",
			from: "v1.34.1",
			to:   "v1.35.1",
			want: []string{"v1.34.1", "v1.35.1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			chain, err := upgradepath.ResolveChain(fsys, "OnPremises", tt.from, tt.to)
			require.NoError(t, err, "ResolveChain")
			assert.Equal(t, tt.want, chainVersions(chain), "chain")
		})
	}
}

func TestResolveChainPrefersHigherIntermediates(t *testing.T) {
	t.Parallel()

	// Both 1.33.0 and 1.33.1 reach the target in the same number of hops. The planner
	// must take the higher one so the cluster picks up the latest patch on the way.
	fsys := fsWith(
		"onpremises",
		"1.32.0-1.33.0",
		"1.32.0-1.33.1",
		"1.33.0-1.34.1",
		"1.33.1-1.34.1",
	)

	chain, err := upgradepath.ResolveChain(fsys, "OnPremises", "1.32.0", "1.34.1")
	require.NoError(t, err, "ResolveChain")
	assert.Equal(t, []string{"v1.32.0", "v1.33.1", "v1.34.1"}, chainVersions(chain), "chain")
}

func TestResolveChainErrors(t *testing.T) {
	t.Parallel()

	fsys := fsWith("onpremises", "1.32.0-1.33.1", "1.33.1-1.34.1")

	t.Run("no path to the target names where the version can actually go", func(t *testing.T) {
		t.Parallel()

		_, err := upgradepath.ResolveChain(fsys, "OnPremises", "1.32.0", "1.99.0")

		require.ErrorIs(t, err, upgradepath.ErrNoChain, "error kind")
		assert.Contains(t, err.Error(), "v1.33.1", "the error must name the reachable versions")
	})

	t.Run("a version with no outgoing hop reports that plainly", func(t *testing.T) {
		t.Parallel()

		_, err := upgradepath.ResolveChain(fsys, "OnPremises", "1.34.1", "1.35.1")

		require.ErrorIs(t, err, upgradepath.ErrNoChain, "error kind")
		assert.Contains(t, err.Error(), "nothing", "a dead end must say so")
	})

	t.Run("an unknown kind is reported as such", func(t *testing.T) {
		t.Parallel()

		_, err := upgradepath.ResolveChain(fsys, "NotAKind", "1.32.0", "1.33.1")

		require.ErrorIs(t, err, upgradepath.ErrNoUpgradePaths, "error kind")
	})
}

func TestResolveChainSkipsMalformedDirectories(t *testing.T) {
	t.Parallel()

	fsys := fsWith("onpremises", "1.32.0-1.33.1", "notaversion", "1.33.1-1.34.1-1.35.1", "-1.34.1")

	// The usable hop still resolves, the unusable names are ignored rather than guessed at.
	chain, err := upgradepath.ResolveChain(fsys, "OnPremises", "1.32.0", "1.33.1")
	require.NoError(t, err, "ResolveChain")
	assert.Equal(t, []string{"v1.32.0", "v1.33.1"}, chainVersions(chain), "chain")

	assert.Equal(t, []string{"v1.33.1"}, upgradepath.Next(fsys, "OnPremises", "1.32.0"), "Next")
}

// TestResolveChainAgainstShippedPaths runs the planner over the upgrade paths furyctl
// actually ships, rather than a fixture, so that a change to that data is noticed here.
func TestResolveChainAgainstShippedPaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		kind string
		from string
		to   string
	}{
		{kind: "OnPremises", from: "1.34.0", to: "1.35.1"},
		{kind: "OnPremises", from: "1.32.0", to: "1.35.1"},
		{kind: "KFDDistribution", from: "1.32.0", to: "1.35.1"},
	}

	for _, tt := range tests {
		t.Run(tt.kind+" "+tt.from+" to "+tt.to, func(t *testing.T) {
			t.Parallel()

			chain, err := upgradepath.ResolveChain(configs.Tpl, tt.kind, tt.from, tt.to)
			require.NoError(t, err, "ResolveChain")
			require.NotEmpty(t, chain, "chain")

			// The chain must start where asked, end where asked, and be contiguous:
			// every hop has to start from the version the previous one produced.
			assert.Equal(t, "v"+tt.from, chain[0].From, "chain start")
			assert.Equal(t, "v"+tt.to, chain[len(chain)-1].To, "chain end")

			for i := 1; i < len(chain); i++ {
				assert.Equal(t, chain[i-1].To, chain[i].From, "hop %d must continue from hop %d", i, i-1)
			}

			// Every hop must be one furyctl actually ships.
			for _, hop := range chain {
				assert.Contains(t, upgradepath.Next(configs.Tpl, tt.kind, hop.From), hop.To,
					"hop %s -> %s must be a shipped upgrade path", hop.From, hop.To)
			}
		})
	}
}

// TestShippedPathsAreAllUsable holds for whatever upgrade paths the distribution ships, now
// and in the future. Pinning a particular version's paths instead would make this test a
// record of one release rather than a check of the resolver.
func TestShippedPathsAreAllUsable(t *testing.T) {
	t.Parallel()

	for _, kind := range []string{"OnPremises", "EKSCluster", "KFDDistribution", "Immutable"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()

			entries, err := fs.ReadDir(configs.Tpl, "upgrades/"+strings.ToLower(kind))
			require.NoError(t, err, "reading the shipped upgrade paths")
			require.NotEmpty(t, entries, "the kind ships no upgrade path at all")

			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}

				from, to, ok := strings.Cut(entry.Name(), "-")
				require.True(t, ok, "%s is not shaped like <from>-<to>", entry.Name())

				// Every shipped hop must be reachable through the resolver, otherwise the
				// directory exists but no upgrade can ever use it.
				assert.Contains(t, upgradepath.Next(configs.Tpl, kind, from), "v"+to,
					"%s ships the hop %s but the resolver does not offer it", kind, entry.Name())

				chain, err := upgradepath.ResolveChain(configs.Tpl, kind, from, to)
				require.NoError(t, err, "resolving the shipped hop %s", entry.Name())
				require.Len(t, chain, 1, "a shipped hop must resolve to itself in one step")
			}
		})
	}
}
