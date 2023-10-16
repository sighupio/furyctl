// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package diff

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	diffx "github.com/r3labs/diff/v3"
)

var (
	numbersToWildcardRegex = regexp.MustCompile(`\.\d+\b`)
	errImmutable           = errors.New("immutable value changed")
)

type Checker interface {
	AssertImmutableViolations(diffs diffx.Changelog, immutablePaths []string) []error
	GenerateDiff() (diffx.Changelog, error)
}

type BaseChecker struct {
	currentConfig map[string]any
	newConfig     map[string]any
}

func NewBaseChecker(currentConfig, newConfig map[string]any) *BaseChecker {
	return &BaseChecker{
		currentConfig: currentConfig,
		newConfig:     newConfig,
	}
}

func (v *BaseChecker) GenerateDiff() (diffx.Changelog, error) {
	changelog, err := diffx.Diff(v.currentConfig, v.newConfig)
	if err != nil {
		return nil, fmt.Errorf("error while diffing configs: %w", err)
	}

	return changelog, nil
}

func (*BaseChecker) AssertImmutableViolations(diffs diffx.Changelog, immutablePaths []string) []error {
	var errs []error

	if len(diffs) == 0 {
		return nil
	}

	for _, diff := range diffs {
		if immutableHelper(diff, immutablePaths) {
			errs = append(
				errs,
				fmt.Errorf(
					"%w: path %s  oldValue %v newValue %v",
					errImmutable,
					"."+strings.Join(diff.Path, "."),
					diff.From,
					diff.To,
				),
			)
		}
	}

	return errs
}

func immutableHelper(change diffx.Change, immutables []string) bool {
	joinedPath := "." + strings.Join(change.Path, ".")
	changePath := numbersToWildcardRegex.ReplaceAllString(joinedPath, ".*")

	for _, immutable := range immutables {
		if changePath == immutable {
			return true
		}
	}

	return false
}
