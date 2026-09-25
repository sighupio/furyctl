// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package upgradepath resolves the upgrade paths that furyctl ships as
// upgrades/<kind>/<from>-<to> directories, so that an upgrade spanning several
// distribution versions can be planned as a sequence of supported hops.
package upgradepath

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"

	"github.com/sighupio/furyctl/internal/semver"
)

const (
	upgradesDir = "upgrades"
	// Separator between the source and target versions in an upgrade directory name.
	hopSeparator = "-"
	// Number of versions an upgrade directory name holds.
	hopParts = 2
)

var (
	// ErrNoUpgradePaths is returned when furyctl ships no upgrade paths for a kind.
	ErrNoUpgradePaths = errors.New("no upgrade paths available for the kind")
	// ErrNoChain is returned when no sequence of supported hops reaches the target.
	ErrNoChain = errors.New("no upgrade path found")
)

// Hop is a single supported upgrade step between two distribution versions.
// Both versions carry the "v" prefix.
type Hop struct {
	From string
	To   string
}

// Next returns the distribution versions directly reachable from the given one,
// ordered from the lowest to the highest and carrying the "v" prefix. It returns
// nil when the kind is unknown or the version is a dead end.
func Next(fsys fs.FS, kind, from string) []string {
	edges, err := readEdges(fsys, kind)
	if err != nil {
		return nil
	}

	return prefixedSorted(edges[semver.EnsureNoPrefix(from)])
}

// ResolveChain returns the shortest sequence of supported hops upgrading a cluster of
// the given kind from one distribution version to another. When several shortest chains
// exist, the one going through the highest intermediate versions is returned, so that a
// cluster picks up the latest patch available at every step.
//
// An empty chain and no error means the cluster is already at the target version.
func ResolveChain(fsys fs.FS, kind, from, to string) ([]Hop, error) {
	edges, err := readEdges(fsys, kind)
	if err != nil {
		return nil, err
	}

	src := semver.EnsureNoPrefix(from)
	dst := semver.EnsureNoPrefix(to)

	if src == dst {
		return []Hop{}, nil
	}

	// Distance of every version from the target, walking the graph backwards. Having
	// it lets the forward walk below always pick a step that makes progress, which is
	// what keeps the resulting chain the shortest one.
	dist := distancesTo(edges, dst)

	if _, reachable := dist[src]; !reachable {
		return nil, fmt.Errorf(
			"%w for kind %s from %s to %s: %s can only be upgraded to %s",
			ErrNoChain,
			kind,
			semver.EnsurePrefix(src),
			semver.EnsurePrefix(dst),
			semver.EnsurePrefix(src),
			reachableList(edges, src),
		)
	}

	chain := make([]Hop, 0, dist[src])

	for cur := src; cur != dst; {
		next, ok := bestStep(edges[cur], dist, dist[cur])
		if !ok {
			return nil, fmt.Errorf("%w: chain breaks at %s", ErrNoChain, semver.EnsurePrefix(cur))
		}

		chain = append(chain, Hop{From: semver.EnsurePrefix(cur), To: semver.EnsurePrefix(next)})
		cur = next
	}

	return chain, nil
}

// readEdges builds the adjacency list of supported hops for a kind from the
// upgrades/<kind>/<from>-<to> directory names.
func readEdges(fsys fs.FS, kind string) (map[string][]string, error) {
	dir := path.Join(upgradesDir, strings.ToLower(kind))

	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("%w %s: %w", ErrNoUpgradePaths, kind, err)
	}

	edges := map[string][]string{}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		from, to, ok := parseHopDir(entry.Name())
		if !ok {
			continue
		}

		edges[from] = append(edges[from], to)
	}

	if len(edges) == 0 {
		return nil, fmt.Errorf("%w %s", ErrNoUpgradePaths, kind)
	}

	return edges, nil
}

// parseHopDir splits a "<from>-<to>" directory name. A name that does not hold exactly
// two versions is reported as unusable rather than guessed at, so that an unexpected
// entry is skipped instead of producing a bogus hop.
func parseHopDir(name string) (string, string, bool) {
	parts := strings.Split(name, hopSeparator)
	if len(parts) != hopParts {
		return "", "", false
	}

	if parts[0] == "" || parts[1] == "" {
		return "", "", false
	}

	return parts[0], parts[1], true
}

// distancesTo returns, for every version that can reach the target, how many hops it
// takes to get there.
func distancesTo(edges map[string][]string, target string) map[string]int {
	reverse := map[string][]string{}

	for from, targets := range edges {
		for _, to := range targets {
			reverse[to] = append(reverse[to], from)
		}
	}

	dist := map[string]int{target: 0}
	queue := []string{target}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		for _, prev := range reverse[cur] {
			if _, seen := dist[prev]; seen {
				continue
			}

			dist[prev] = dist[cur] + 1

			queue = append(queue, prev)
		}
	}

	return dist
}

// bestStep picks the highest version among the neighbours that get one hop closer to
// the target.
func bestStep(targets []string, dist map[string]int, curDist int) (string, bool) {
	best := ""
	found := false

	for _, target := range targets {
		d, known := dist[target]
		if !known || d != curDist-1 {
			continue
		}

		if !found || compareVersions(target, best) > 0 {
			best = target
			found = true
		}
	}

	return best, found
}

// compareVersions orders two versions. It falls back to a plain string comparison when
// either side is not valid semver, so that an unexpected directory name degrades to a
// stable order instead of an error.
func compareVersions(a, b string) int {
	va, errA := semver.NewVersion(a)
	vb, errB := semver.NewVersion(b)

	if errA != nil || errB != nil {
		return strings.Compare(a, b)
	}

	return va.Compare(vb)
}

// prefixedSorted returns the versions with the "v" prefix, lowest first.
func prefixedSorted(versions []string) []string {
	if len(versions) == 0 {
		return nil
	}

	out := make([]string, 0, len(versions))

	for _, v := range versions {
		out = append(out, semver.EnsurePrefix(v))
	}

	slices.SortFunc(out, compareVersions)

	return out
}

// reachableList renders the versions directly reachable from one version, for error
// messages that tell the user where they can actually go.
func reachableList(edges map[string][]string, from string) string {
	targets := prefixedSorted(edges[from])
	if len(targets) == 0 {
		return "nothing"
	}

	return strings.Join(targets, ", ")
}
