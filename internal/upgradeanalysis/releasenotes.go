// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package upgradeanalysis

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/sighupio/furyctl/internal/semver"
)

// ErrNoBreakingChangesSection is returned when a release ships notes without a breaking
// changes section. That is not the same as a release with no breaking changes: some releases
// state "None" explicitly, others omit the section, and only the release itself knows which.
var (
	ErrNoBreakingChangesSection = errors.New("no release notes cover this hop")

	// Matches the breaking changes heading. Releases have spelled it both "Breaking changes"
	// and "Breaking Changes", and decorate it with an emoji, so the match is case-insensitive
	// and ignores whatever follows on the line.
	breakingChangesHeading = regexp.MustCompile(`(?i)^##\s+breaking\s+changes?\b`)

	// Matches the start of the next section, which ends the one being read.
	anyHeading = regexp.MustCompile(`^##\s`)
)

// ReleaseBreakingChanges is what one release declares breaking, in its own words.
type ReleaseBreakingChanges struct {
	Version string `json:"version" yaml:"version"`
	// Section is the release's breaking changes, verbatim. Empty means the release ships
	// notes without such a section, which is not the same as declaring none: some releases
	// say "None" explicitly, and only the release itself knows which it meant.
	Section string `json:"section,omitempty" yaml:"section,omitempty"`
}

// breakingChangesBetween returns what every release in (from, to] declares breaking.
//
// A hop can skip releases: an upgrade path may jump over a patch that is never installed,
// while everything that patch declared breaking still applies to the cluster. Reading only
// the target's notes would hide exactly the changes an upgrade most needs to know about.
//
// The notes of every earlier release ship inside the target's distribution, so this needs no
// extra download and stays correct for versions that do not exist yet.
func breakingChangesBetween(distributionPath, from, to string) ([]ReleaseBreakingChanges, error) {
	dir := filepath.Join(distributionPath, "docs", "releases")

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("cannot read the release notes: %w", err)
	}

	versions, err := versionsInSpan(entries, from, to)
	if err != nil {
		return nil, err
	}

	out := make([]ReleaseBreakingChanges, 0, len(versions))

	for _, version := range versions {
		raw, readErr := os.ReadFile(filepath.Join(dir, version+".md"))
		if readErr != nil {
			return nil, fmt.Errorf("cannot read the release notes of %s: %w", version, readErr)
		}

		out = append(out, ReleaseBreakingChanges{
			Version: version,
			Section: extractSection(string(raw)),
		})
	}

	if len(out) == 0 {
		return nil, ErrNoBreakingChangesSection
	}

	return out, nil
}

// versionsInSpan selects the release notes covering (from, to], oldest first.
func versionsInSpan(entries []os.DirEntry, from, to string) ([]string, error) {
	lower, err := semver.NewVersion(from)
	if err != nil {
		return nil, fmt.Errorf("cannot read the version %s: %w", from, err)
	}

	upper, err := semver.NewVersion(to)
	if err != nil {
		return nil, fmt.Errorf("cannot read the version %s: %w", to, err)
	}

	var versions []string

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".md")

		parsed, parseErr := semver.NewVersion(name)
		if parseErr != nil {
			continue
		}

		if parsed.GreaterThan(lower) && !parsed.GreaterThan(upper) {
			versions = append(versions, name)
		}
	}

	slices.SortFunc(versions, func(a, b string) int {
		va, errA := semver.NewVersion(a)
		vb, errB := semver.NewVersion(b)

		if errA != nil || errB != nil {
			return strings.Compare(a, b)
		}

		return va.Compare(vb)
	})

	return versions, nil
}

// extractSection returns the breaking changes section of a release notes document, from its
// heading to the next one.
func extractSection(notes string) string {
	lines := strings.Split(notes, "\n")

	start := -1

	for i, line := range lines {
		if breakingChangesHeading.MatchString(line) {
			start = i + 1

			break
		}
	}

	if start < 0 {
		return ""
	}

	end := len(lines)

	for i := start; i < len(lines); i++ {
		if anyHeading.MatchString(lines[i]) {
			end = i

			break
		}
	}

	return strings.TrimSpace(strings.Join(lines[start:end], "\n"))
}
