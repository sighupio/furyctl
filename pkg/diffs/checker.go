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

	rules "github.com/sighupio/furyctl/pkg/rulesextractor"
)

var (
	numbersToWildcardRegex = regexp.MustCompile(`\.\d+\b`)
	errImmutable           = errors.New("immutable value changed")
	errUnsupported         = errors.New("unsupported value changed")
)

type Checker interface {
	AssertImmutableViolations(diffs r3diff.Changelog, immutablePaths []string) []error
	AssertReducerUnsupportedViolations(diffs r3diff.Changelog, reducerRules []rules.Rule) []error
	GenerateDiff() (r3diff.Changelog, error)
	DiffToString(diffs r3diff.Changelog) string
	FilterDiffFromPhase(changelog r3diff.Changelog, phasePath string) r3diff.Changelog
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

func (*BaseChecker) AssertReducerUnsupportedViolations(diffs r3diff.Changelog, reducerRules []rules.Rule) []error {
	var errs []error

	if len(diffs) == 0 || len(reducerRules) == 0 {
		return nil
	}

	for _, diff := range diffs {
		for _, rule := range reducerRules {
			joinedPath := "." + strings.Join(diff.Path, ".")
			changePath := numbersToWildcardRegex.ReplaceAllString(joinedPath, ".*")

			if rule.Path == changePath {
				if rule.Unsupported != nil && len(*rule.Unsupported) > 0 {
					if reason, unsupported := isDiffUnsupported(diff, *rule.Unsupported); unsupported {
						unsupportedGenericErrMsg := fmt.Sprintf(
							"changing %s from %v to %v is not supported",
							changePath,
							diff.From,
							diff.To,
						)

						if reason != "" {
							unsupportedGenericErrMsg = reason
						}

						errs = append(errs, fmt.Errorf("%w: %s", errUnsupported, unsupportedGenericErrMsg))
					}
				}
			}
		}
	}

	return errs
}

func isDiffUnsupported(diff r3diff.Change, conditions []rules.Unsupported) (string, bool) {
	reason := ""

	for _, condition := range conditions {
		if (condition.From == nil || (condition.From != nil && diff.From == *condition.From)) &&
			(condition.To == nil || (condition.To != nil && diff.To == *condition.To)) {
			if condition.Reason != nil {
				reason = *condition.Reason
			}

			return reason, true
		}
	}

	return reason, false
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
