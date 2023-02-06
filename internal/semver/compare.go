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
	// Link: https://regex101.com/r/WrEwCK/1
	//nolint:lll //We can't wrap regex
	regex                = regexp.MustCompile(`^(\*)$|^((~|\^|<=|<|>|>=)?(?P<major>0|[1-9]\d*)\.(?P<minor>0|[1-9]\d*)\.(?P<patch>0|[1-9]\d*)(?:-(?P<prerelease>(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+(?P<buildmetadata>[0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?)$`)
	comparatorsRegex     = regexp.MustCompile(`^(~|\^|<=|>=|<|>|\*)`)
	ErrInvalidSemver     = fmt.Errorf("invalid semantic version")
	ErrInvalidComparator = fmt.Errorf("invalid comparator")
)

type Comparer interface {
	Compare(a, b Version) bool
}

func NewComparer(c string) (Comparer, error) {
	switch c {
	case "~":
		return CompatibleUp{}, nil

	case "^":
		return Compatible{}, nil

	case "<=":
		return LessOrEqual{}, nil

	case "<":
		return Less{}, nil

	case ">=":
		return GreaterOrEqual{}, nil

	case ">":
		return Greater{}, nil

	case "*":
		return Always{}, nil

	case "":
		return Equal{}, nil

	default:
		return nil, fmt.Errorf("%w: %s", ErrInvalidComparator, c)
	}
}

type Always struct{}

func (Always) Compare(_, _ Version) bool {
	return true
}

type LessOrEqual struct{}

func (LessOrEqual) Compare(a, b Version) bool {
	gt := Gt(b.String(), a.String())

	if gt {
		return true
	}

	return a.String() == b.String()
}

type Less struct{}

func (Less) Compare(a, b Version) bool {
	return Gt(b.String(), a.String())
}

type GreaterOrEqual struct{}

func (GreaterOrEqual) Compare(a, b Version) bool {
	gt := Gt(a.String(), b.String())

	if gt {
		return true
	}

	return a.String() == b.String()
}

type Equal struct{}

func (Equal) Compare(a, b Version) bool {
	return a.String() == b.String()
}

type Greater struct{}

func (Greater) Compare(a, b Version) bool {
	return Gt(a.String(), b.String())
}

type Compatible struct{}

func (Compatible) Compare(a, b Version) bool {
	aParts := Parts(a.String())
	bParts := Parts(b.String())

	return aParts.Major == bParts.Major
}

type CompatibleUp struct{}

func (CompatibleUp) Compare(a, b Version) bool {
	return SameMinor(a, b)
}

// NewVersion takes a string and returns a Version.
func NewVersion(v string) (Version, error) {
	if !isValid(v) {
		return "", fmt.Errorf("%w: %s", ErrInvalidSemver, v)
	}

	return Version(v), nil
}

type VersionParts struct {
	Comparator Comparer
	Major      int
	Minor      int
	Patch      int
	Suffix     string
}

func (v VersionParts) String() string {
	if v.Suffix != "" {
		return fmt.Sprintf("%d.%d.%d-%s", v.Major, v.Minor, v.Patch, v.Suffix)
	}

	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

func (v VersionParts) CheckCompatibility(b VersionParts) bool {
	if v.Comparator == nil {
		return v.String() == b.String()
	}

	return v.Comparator.Compare(Version(b.String()), Version(v.String()))
}

type Version string

func (s Version) String() string {
	return string(s)
}

// SamePatchStr takes two version strings and tell if they match down to patch level.
func SamePatchStr(a, b string) bool {
	return SamePatch(Version(a), Version(b))
}

// SamePatch takes two versions and tell if they are part of the same patch.
func SamePatch(a, b Version) bool {
	if a == b {
		return true
	}

	aParts := Parts(a.String())
	bParts := Parts(b.String())

	return aParts.Major == bParts.Major && aParts.Minor == bParts.Minor && aParts.Patch == bParts.Patch
}

// SameMinorStr takes two version strings and tell if they match down to minor level.
func SameMinorStr(a, b string) bool {
	return SameMinor(Version(a), Version(b))
}

// SameMinor takes two versions and tell if they are part of the same minor.
func SameMinor(a, b Version) bool {
	if a == b {
		return true
	}

	aParts := Parts(a.String())
	bParts := Parts(b.String())

	return aParts.Major == bParts.Major && aParts.Minor == bParts.Minor
}

// Gt returns true if a is greater than b.
func Gt(va, vb string) bool {
	if va == vb {
		return false
	}

	aParts := Parts(va)
	bParts := Parts(vb)

	if aParts.Major > bParts.Major {
		return true
	}

	if aParts.Major < bParts.Major {
		return false
	}

	if aParts.Minor > bParts.Minor {
		return true
	}

	if aParts.Minor < bParts.Minor {
		return false
	}

	if aParts.Patch > bParts.Patch {
		return true
	}

	if aParts.Patch < bParts.Patch {
		return false
	}

	return false
}

// Parts return the comparer, major, minor, patch and build+prerelease parts of a version.
func Parts(v string) VersionParts {
	pv := EnsurePrefix(v)

	if !isValid(pv) {
		return VersionParts{
			Comparator: nil,
			Major:      0,
			Minor:      0,
			Patch:      0,
			Suffix:     "",
		}
	}

	cmpStrIndex := comparatorsRegex.FindStringIndex(v)
	cmpStr := ""
	vStr := v

	if len(cmpStrIndex) > 0 {
		cmpStr = v[cmpStrIndex[0]:cmpStrIndex[1]]
		vStr = v[cmpStrIndex[1]:]
	}

	cmp, err := NewComparer(cmpStr)
	if err != nil {
		return VersionParts{
			Comparator: nil,
			Major:      0,
			Minor:      0,
			Patch:      0,
			Suffix:     "",
		}
	}

	if len(vStr) == 0 {
		return VersionParts{
			Comparator: cmp,
			Major:      0,
			Minor:      0,
			Patch:      0,
			Suffix:     "",
		}
	}

	parts := strings.Split(EnsureNoPrefix(vStr), ".")

	ch := "-"
	m := strings.Index(vStr, "-")
	p := strings.Index(vStr, "+")

	if (m == -1 && p > -1) || (m > -1 && p > -1 && p < m) {
		ch = "+"
	}

	patchParts := strings.Split(strings.Join(parts[2:], "."), ch)

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return VersionParts{
			Comparator: nil,
			Major:      0,
			Minor:      0,
			Patch:      0,
			Suffix:     "",
		}
	}

	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return VersionParts{
			Comparator: nil,
			Major:      0,
			Minor:      0,
			Patch:      0,
			Suffix:     "",
		}
	}

	patch, err := strconv.Atoi(patchParts[0])
	if err != nil {
		return VersionParts{
			Comparator: nil,
			Major:      0,
			Minor:      0,
			Patch:      0,
			Suffix:     "",
		}
	}

	if len(patchParts) > 1 {
		return VersionParts{
			Comparator: cmp,
			Major:      major,
			Minor:      minor,
			Patch:      patch,
			Suffix:     strings.Join(patchParts[1:], ch),
		}
	}

	return VersionParts{
		Comparator: cmp,
		Major:      major,
		Minor:      minor,
		Patch:      patch,
		Suffix:     "",
	}
}

func isValid(v string) bool {
	if v[0] != 'v' {
		return false
	}

	return regex.Match([]byte(v[1:]))
}
