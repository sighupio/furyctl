// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package semver

import (
	"fmt"

	"github.com/Al-Pragliola/go-version"
)

var (
	// ErrInvalidVersion is returned when the version is not valid.
	ErrInvalidVersion = fmt.Errorf("invalid version")
	// ErrInvalidConstraint is returned when the constraint is not valid.
	ErrInvalidConstraint = fmt.Errorf("invalid constraint")
)

func GetVersion(v string) (*version.Version, error) {
	vStr := v

	if v[0] == 'v' {
		vStr = v[1:]
	}

	ver, err := version.NewVersion(vStr)
	if err != nil {
		return nil, ErrInvalidVersion
	}

	return ver, nil
}

func GetConstraint(c string) (version.Constraints, error) {
	cStr := c

	if c[0] == 'v' {
		cStr = c[1:]
	}

	cnst, err := version.NewConstraint(cStr)
	if err != nil {
		return nil, ErrInvalidConstraint
	}

	return cnst, nil
}
