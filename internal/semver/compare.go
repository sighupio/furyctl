// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package semver

import (
	"errors"
	"fmt"

	"github.com/Al-Pragliola/go-version"
)

var (
	// ErrInvalidVersion is returned when the version is not valid.
	ErrInvalidVersion = errors.New("invalid version")
	// ErrInvalidConstraint is returned when the constraint is not valid.
	ErrInvalidConstraint = errors.New("invalid constraint")
)

func NewVersion(v string) (*version.Version, error) {
	if len(v) == 0 {
		return nil, fmt.Errorf("%w: version is empty", ErrInvalidVersion)
	}

	vStr := v

	if v[0] == 'v' {
		vStr = v[1:]
	}

	ver, err := version.NewVersion(vStr)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidVersion, err)
	}

	return ver, nil
}

func NewConstraint(c string) (version.Constraints, error) {
	if len(c) == 0 {
		return nil, fmt.Errorf("%w: constraint is empty", ErrInvalidConstraint)
	}

	cStr := c

	if c[0] == 'v' {
		cStr = c[1:]
	}

	cnst, err := version.NewConstraint(cStr)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidConstraint, err)
	}

	return cnst, nil
}
