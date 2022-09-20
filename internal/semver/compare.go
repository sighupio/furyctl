// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package semver

import (
	"fmt"
	"strconv"
	"strings"
)

var ErrInvalidSemver = fmt.Errorf("invalid semantic version")

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

// SamePatch takes two versions and tell if they are part of the same patch
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

// SameMinor takes two versions and tell if they are part of the same minor
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
	if !strings.HasPrefix(v, "v") {
		return false
	}

	v = strings.TrimPrefix(v, "v")

	parts := strings.Split(v, ".")

	if len(parts) != 3 {
		return false
	}

	if _, err := strconv.Atoi(parts[0]); err != nil {
		return false
	}

	if _, err := strconv.Atoi(parts[1]); err != nil {
		return false
	}

	if _, err := strconv.Atoi(parts[2]); err != nil {
		return false
	}

	return true

}
