// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package semver

import (
	"fmt"
	"regexp"
	"strconv"
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

	aMajor, aMinor, aPatch, _ := Parts(a.String())
	bMajor, bMinor, bPatch, _ := Parts(b.String())

	return aMajor == bMajor && aMinor == bMinor && aPatch == bPatch
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

	aMajor, aMinor, _, _ := Parts(a.String())
	bMajor, bMinor, _, _ := Parts(b.String())

	return aMajor == bMajor && aMinor == bMinor
}

// Gt returns true if a is greater than b
func Gt(va, vb string) bool {
	if va == vb {
		return false
	}

	aMajor, aMinor, aPatch, _ := Parts(va)
	bMajor, bMinor, bPatch, _ := Parts(vb)

	if aMajor > bMajor {
		return true
	}

	if aMajor < bMajor {
		return false
	}

	if aMinor > bMinor {
		return true
	}

	if aMinor < bMinor {
		return false
	}

	if aPatch > bPatch {
		return true
	}

	if aPatch < bPatch {
		return false
	}

	return false
}

// Parts returns the major, minor, patch and buil+prerelease parts of a version
func Parts(v string) (int, int, int, string) {
	pv := EnsurePrefix(v)

	if !isValid(pv) {
		return 0, 0, 0, ""
	}

	parts := strings.Split(EnsureNoPrefix(v), ".")

	ch := "-"
	m := strings.Index(v, "-")
	p := strings.Index(v, "+")
	if (m == -1 && p > -1) || (m > -1 && p > -1 && p < m) {
		ch = "+"
	}

	patchParts := strings.Split(strings.Join(parts[2:], "."), ch)

	major, _ := strconv.Atoi(parts[0])
	minor, _ := strconv.Atoi(parts[1])
	patch, _ := strconv.Atoi(patchParts[0])

	if len(patchParts) > 1 {
		return major, minor, patch, strings.Join(patchParts[1:], ch)
	}

	return major, minor, patch, ""
}

func isValid(v string) bool {
	if v[0] != 'v' {
		return false
	}

	return regex.Match([]byte(v[1:]))
}
