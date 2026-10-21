// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package helmfile

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type ChangeKind string

const (
	ChangeCreate ChangeKind = "create"
	ChangeUpdate ChangeKind = "update"
	ChangeDelete ChangeKind = "delete"

	reasonFormat    = "helmfile diff output did not match the expected format"
	reasonOwnership = "helm-diff reports an ownership change"
	reasonDuplicate = "helmfile diff printed this release more than once"
	reasonMissing   = "helmfile diff printed no result for this release"

	// The capture groups of resourceLineRe.
	groupKind = 3
	groupAPI  = 4
	groupVerb = 5

	// The number of counts of a plan line, and the index of the last one.
	planCounts     = 4
	ownershipIndex = planCounts - 1
)

// ResourceChange is one resource line of a release block.
type ResourceChange struct {
	Namespace string
	Name      string
	Kind      string
	API       string
	Change    ChangeKind
}

// ReleaseDiff is the comparison result of one release. A release that NotCompared has no changes.
type ReleaseDiff struct {
	Name        string
	Changes     []ResourceChange
	NotCompared bool
	Reason      string
}

// HelmDiffResult is the comparison result of the whole helm section. A Reason means that the section was not compared.
type HelmDiffResult struct {
	Releases []ReleaseDiff
	Reason   string
}

// Failed reports whether the section, or any of its releases, was not compared.
func (r HelmDiffResult) Failed() bool {
	if r.Reason != "" {
		return true
	}

	for _, rel := range r.Releases {
		if rel.NotCompared {
			return true
		}
	}

	return false
}

var (
	releaseHeaderRe = regexp.MustCompile(`^Comparing release=([^,]+), chart=.*$`)
	resourceLineRe  = regexp.MustCompile(
		`^([^,]*), (.+), (\S+) \(([^)]*)\) ` +
			`(to be added\.|to be changed\.|has changed, but diff is empty after suppression\.|` +
			`to be removed\.|to change ownership\.)$`,
	)
	planLineRe = regexp.MustCompile(
		`^Plan: (\d+) to add, (\d+) to change, (\d+) to destroy(?:, (\d+) to change ownership)?\.$`,
	)
)

// diffBlock is the raw content of one `Comparing release=...` block.
type diffBlock struct {
	changes   []ResourceChange
	listed    plan
	declared  *plan
	malformed bool
}

type plan struct {
	add, change, destroy, ownership int
}

// ParseHelmDiff reads the output of `helmfile diff --output simple` into one result per configured release.
func ParseHelmDiff(stdout, stderr string, code int, releases []string) HelmDiffResult {
	if code != 0 {
		return HelmDiffResult{Reason: fmt.Sprintf("helmfile diff exited %d: %s", code, errorHint(stderr))}
	}

	blocks := splitBlocks(stdout)

	result := HelmDiffResult{}

	for _, name := range releases {
		result.Releases = append(result.Releases, releaseResult(name, blocks[name]))
	}

	return result
}

func splitBlocks(stdout string) map[string][]*diffBlock {
	blocks := map[string][]*diffBlock{}

	var current *diffBlock

	for raw := range strings.SplitSeq(stdout, "\n") {
		line := strings.TrimRight(raw, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}

		if m := releaseHeaderRe.FindStringSubmatch(line); m != nil {
			current = &diffBlock{}
			blocks[m[1]] = append(blocks[m[1]], current)

			continue
		}

		if current != nil {
			current.readLine(line)
		}
	}

	return blocks
}

func (b *diffBlock) readLine(line string) {
	if b.declared != nil {
		b.malformed = true

		return
	}

	if m := planLineRe.FindStringSubmatch(line); m != nil {
		// A plan line whose counts cannot be read leaves the block without a plan.
		p, err := parsePlan(m)
		if err != nil {
			return
		}

		b.declared = &p

		return
	}

	m := resourceLineRe.FindStringSubmatch(line)
	if m == nil {
		b.malformed = true

		return
	}

	b.addResource(m)
}

func (b *diffBlock) addResource(m []string) {
	rc := ResourceChange{Namespace: m[1], Name: m[2], Kind: m[groupKind], API: m[groupAPI]}

	switch m[groupVerb] {
	case "to be added.":
		rc.Change = ChangeCreate
		b.listed.add++
	case "to be changed.":
		rc.Change = ChangeUpdate
		b.listed.change++
	case "has changed, but diff is empty after suppression.":
		rc.Change = ChangeUpdate
	case "to be removed.":
		rc.Change = ChangeDelete
		b.listed.destroy++
	default:
		b.listed.ownership++

		return
	}

	b.changes = append(b.changes, rc)
}

func releaseResult(name string, blocks []*diffBlock) ReleaseDiff {
	switch {
	case len(blocks) == 0:
		return notCompared(name, reasonMissing)
	case len(blocks) > 1:
		return notCompared(name, reasonDuplicate)
	}

	b := blocks[0]

	switch {
	case b.malformed || b.declared == nil || *b.declared != b.listed:
		return notCompared(name, reasonFormat)
	case b.listed.ownership > 0:
		return notCompared(name, reasonOwnership)
	}

	return ReleaseDiff{Name: name, Changes: b.changes}
}

func notCompared(name, reason string) ReleaseDiff {
	return ReleaseDiff{Name: name, NotCompared: true, Reason: reason}
}

// errorHint is the first stderr line starting with `Error:` (ignoring leading spaces), else the last non-empty line.
func errorHint(stderr string) string {
	last := ""

	for raw := range strings.SplitSeq(stderr, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}

		if strings.HasPrefix(strings.TrimLeft(raw, " "), "Error:") {
			return line
		}

		last = line
	}

	return last
}

// parsePlan reads the counts of a plan line. The ownership count is absent in the output of helm-diff 3.8.1.
func parsePlan(m []string) (plan, error) {
	counts := [planCounts]int{}

	for i := range counts {
		if m[i+1] == "" {
			continue
		}

		n, err := strconv.Atoi(m[i+1])
		if err != nil {
			return plan{}, fmt.Errorf("error reading the plan counts: %w", err)
		}

		counts[i] = n
	}

	return plan{add: counts[0], change: counts[1], destroy: counts[2], ownership: counts[ownershipIndex]}, nil
}
