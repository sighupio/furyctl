// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package diffs

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	r3diff "github.com/r3labs/diff/v3"
)

var (
	numbersToWildcardRegex = regexp.MustCompile(`\.\d+\b`)
	errImmutable           = errors.New("immutable value changed")
)

type Checker interface {
	AssertImmutableViolations(diffs r3diff.Changelog, immutablePaths []string) []error
	GenerateDiff() (r3diff.Changelog, error)
	DiffToString(diffs r3diff.Changelog) string
}

type BaseChecker struct {
	CurrentConfig map[string]any
	NewConfig     map[string]any
}

func NewBaseChecker(currentConfig, newConfig map[string]any) *BaseChecker {
	return &BaseChecker{
		CurrentConfig: currentConfig,
		NewConfig:     newConfig,
	}
}

func (v *BaseChecker) GenerateDiff() (r3diff.Changelog, error) {
	changelog, err := r3diff.Diff(v.CurrentConfig, v.NewConfig)
	if err != nil {
		return nil, fmt.Errorf("error while diffing configs: %w", err)
	}

	return changelog, nil
}

func (*BaseChecker) FilterDiffFromPhase(changelog r3diff.Changelog, phasePath string) r3diff.Changelog {
	var filteredChangelog r3diff.Changelog

	for _, diff := range changelog {
		joinedPath := "." + strings.Join(diff.Path, ".")

		if strings.HasPrefix(joinedPath, phasePath) {
			filteredChangelog = append(filteredChangelog, diff)
		}
	}

	return filteredChangelog
}

func (*BaseChecker) DiffToString(diffs r3diff.Changelog) string {
	var str string

	for _, diff := range diffs {
		joinedPath := "." + strings.Join(diff.Path, ".")

		str += fmt.Sprintf("%s: %v -> %v\n", joinedPath, diff.From, diff.To)
	}

	return str
}

func (*BaseChecker) AssertImmutableViolations(diffs r3diff.Changelog, immutablePaths []string) []error {
	var errs []error

	if len(diffs) == 0 {
		return nil
	}

	for _, diff := range diffs {
		if isImmutablePathChanged(diff, immutablePaths) {
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

func isImmutablePathChanged(change r3diff.Change, immutables []string) bool {
	joinedPath := "." + strings.Join(change.Path, ".")
	changePath := numbersToWildcardRegex.ReplaceAllString(joinedPath, ".*")

	for _, immutable := range immutables {
		if changePath == immutable {
			return true
		}
	}

	return false
}
