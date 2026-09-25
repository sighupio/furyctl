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
	"strings"

	"github.com/sighupio/furyctl/internal/semver"
)

// ErrNoBreakingChangesSection is returned when a release ships notes without a breaking
// changes section. That is not the same as a release with no breaking changes: some releases
// state "None" explicitly, others omit the section, and only the release itself knows which.
var (
	ErrNoBreakingChangesSection = errors.New("the release notes list no breaking-changes section")

	// Matches the breaking changes heading. Releases have spelled it both "Breaking changes"
	// and "Breaking Changes", and decorate it with an emoji, so the match is case-insensitive
	// and ignores whatever follows on the line.
	breakingChangesHeading = regexp.MustCompile(`(?i)^##\s+breaking\s+changes?\b`)

	// Matches the start of the next section, which ends the one being read.
	anyHeading = regexp.MustCompile(`^##\s`)
)

// breakingChanges returns the breaking changes a distribution version declares, taken
// verbatim from the release notes that version ships.
//
// Reading them from the distribution rather than restating them here is what keeps the
// analysis correct for versions that do not exist yet: whatever a future release declares
// breaking is what the report will show, with no change to furyctl.
func breakingChanges(distributionPath, version string) (string, error) {
	notes := filepath.Join(
		distributionPath, "docs", "releases", semver.EnsurePrefix(version)+".md",
	)

	raw, err := os.ReadFile(notes)
	if err != nil {
		return "", fmt.Errorf("cannot read the release notes: %w", err)
	}

	section := extractSection(string(raw))
	if section == "" {
		return "", ErrNoBreakingChangesSection
	}

	return section, nil
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
