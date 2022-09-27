// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package semver

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	// Link: https://regex101.com/r/Ly7O1x/3/
	regex = regexp.MustCompile(`^(?P<major>0|[1-9]\d*)\.(?P<minor>0|[1-9]\d*)\.(?P<patch>0|[1-9]\d*)(?:-(?P<prerelease>(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+(?P<buildmetadata>[0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`)

	ErrInvalidSemver = fmt.Errorf("invalid semantic version")
)

// NewVersion takes a string and returns a Version
func NewVersion(v string) (Version, error) {
	if !isValid(v) {
		return "", fmt.Errorf("%w: %s", ErrInvalidSemver, v)
	}

	return Version(v), nil
}

type Version string

func (s Version) String() string {
	return string(s)
}

// SamePatchStr takes two version strings and tell if they match down to patch level
func SamePatchStr(a, b string) bool {
	return SamePatch(Version(a), Version(b))
}

// SamePatchStr takes two versions and tell if they match down to patch level
func SamePatch(a, b Version) bool {
	if a == b {
		return true
	}

	ap := strings.Split(a.String(), ".")
	ma := fmt.Sprintf("%s.%s.%s", ap[0], ap[1], ap[2])

	bp := strings.Split(b.String(), ".")
	mb := fmt.Sprintf("%s.%s.%s", bp[0], bp[1], bp[2])

	return ma == mb
}

// SameMinorStr takes two version strings and tell if they match down to minor level
func SameMinorStr(a, b string) bool {
	return SameMinor(Version(a), Version(b))
}

// SameMinorStr takes two versions and tell if they match down to minor level
func SameMinor(a, b Version) bool {
	if a == b {
		return true
	}

	ap := strings.Split(a.String(), ".")
	ma := fmt.Sprintf("%s.%s", ap[0], ap[1])

	bp := strings.Split(b.String(), ".")
	mb := fmt.Sprintf("%s.%s", bp[0], bp[1])

	return ma == mb
}

func isValid(v string) bool {
	if v[0] != 'v' {
		return false
	}

	return regex.Match([]byte(v[1:]))
}
