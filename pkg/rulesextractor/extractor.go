// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rulesextractor

import (
	"regexp"
	"strings"

	"github.com/r3labs/diff/v3"
)

var numbersToWildcardRegex = regexp.MustCompile(`\.\d+\b`)

type Spec struct {
	Infrastructure *[]Rule `yaml:"infrastructure,omitempty"`
	Kubernetes     *[]Rule `yaml:"kubernetes,omitempty"`
	Distribution   *[]Rule `yaml:"distribution,omitempty"`
}

type Rule struct {
	Path        string         `yaml:"path"`
	Immutable   bool           `yaml:"immutable"`
	Description *string        `yaml:"description,omitempty"`
	Unsupported *[]Unsupported `yaml:"unsupported,omitempty"`
	Safe        *[]Safe        `yaml:"safe,omitempty"`
	Reducers    *[]Reducer     `yaml:"reducers,omitempty"`
}

type Unsupported struct {
	From   *any    `yaml:"from,omitempty"`
	To     *any    `yaml:"to,omitempty"`
	Reason *string `yaml:"reason,omitempty"`
}

type FromNode struct {
	Path  *string `yaml:"path"`
	Value *any    `yaml:"value"`
}

type Safe struct {
	From      *any        `yaml:"from,omitempty"`
	To        *any        `yaml:"to,omitempty"`
	FromNodes *[]FromNode `yaml:"fromNodes,omitempty"`
}

type Reducer struct {
	Key       string      `yaml:"key"`
	Lifecycle string      `yaml:"lifecycle"`
	From      any         `yaml:"from"`
	To        any         `yaml:"to"`
	FromNodes *[]FromNode `yaml:"fromNodes,omitempty"`
}

type Extractor interface {
	GetImmutableRules(phase string) []Rule
	FilterSafeImmutableRules(rules []Rule, ds diff.Changelog) []Rule
	GetReducers(phase string) []Rule
	ReducerRulesByDiffs(reducers []Rule, ds diff.Changelog) []Rule
	UnsupportedReducerRulesByDiffs(rules []Rule, ds diff.Changelog) []Rule
	UnsafeReducerRulesByDiffs(rules []Rule, ds diff.Changelog) []Rule
}

type BaseExtractor struct {
	Spec Spec
}

func NewBaseExtractor(spec Spec) *BaseExtractor {
	return &BaseExtractor{
		Spec: spec,
	}
}

func (b *BaseExtractor) GetImmutables(_ string) []string {
	var immutables []string

	if b.Spec.Infrastructure != nil {
		for _, rule := range *b.Spec.Infrastructure {
			if rule.Immutable {
				immutables = append(immutables, rule.Path)
			}
		}
	}

	if b.Spec.Kubernetes != nil {
		for _, rule := range *b.Spec.Kubernetes {
			if rule.Immutable {
				immutables = append(immutables, rule.Path)
			}
		}
	}

	if b.Spec.Distribution != nil {
		for _, rule := range *b.Spec.Distribution {
			if rule.Immutable {
				immutables = append(immutables, rule.Path)
			}
		}
	}

	return immutables
}

// GetImmutableRules returns all the rules that are marked as immutable.
func (b *BaseExtractor) GetImmutableRules(_ string) []Rule {
	var immutableRules []Rule

	if b.Spec.Infrastructure != nil {
		for _, rule := range *b.Spec.Infrastructure {
			if rule.Immutable {
				immutableRules = append(immutableRules, rule)
			}
		}
	}

	if b.Spec.Kubernetes != nil {
		for _, rule := range *b.Spec.Kubernetes {
			if rule.Immutable {
				immutableRules = append(immutableRules, rule)
			}
		}
	}

	if b.Spec.Distribution != nil {
		for _, rule := range *b.Spec.Distribution {
			if rule.Immutable {
				immutableRules = append(immutableRules, rule)
			}
		}
	}

	return immutableRules
}

func (b *BaseExtractor) FilterSafeImmutableRules(rules []Rule, ds diff.Changelog) []Rule {
	filteredRules := make([]Rule, 0)

	for _, rule := range rules {
		// Create an empty reducer to use with the isReducerSafe function.
		dummyReducer := Reducer{}

		// Find the diff that matches this rule's path.
		for _, d := range ds {
			joinedPath := "." + strings.Join(d.Path, ".")
			changePath := numbersToWildcardRegex.ReplaceAllString(joinedPath, ".*")

			if changePath == rule.Path {
				dummyReducer.From = d.From
				dummyReducer.To = d.To

				break
			}
		}

		// If the rule has safe conditions and they match, skip this rule.
		if rule.Safe != nil && len(*rule.Safe) > 0 {
			if b.isReducerSafe(dummyReducer, *rule.Safe, ds) {
				continue
			}
		}

		filteredRules = append(filteredRules, rule)
	}

	return filteredRules
}

func (b *BaseExtractor) GetReducers(_ string) []Rule {
	var reducers []Rule

	if b.Spec.Infrastructure != nil {
		for _, rule := range *b.Spec.Infrastructure {
			if rule.Reducers != nil {
				reducers = append(reducers, rule)
			}
		}
	}

	if b.Spec.Kubernetes != nil {
		for _, rule := range *b.Spec.Kubernetes {
			if rule.Reducers != nil {
				reducers = append(reducers, rule)
			}
		}
	}

	if b.Spec.Distribution != nil {
		for _, rule := range *b.Spec.Distribution {
			if rule.Reducers != nil {
				reducers = append(reducers, rule)
			}
		}
	}

	return reducers
}

func (*BaseExtractor) ReducerRulesByDiffs(rules []Rule, ds diff.Changelog) []Rule {
	filteredRules := make([]Rule, 0)

	for _, rule := range rules {
		for _, d := range ds {
			joinedPath := "." + strings.Join(d.Path, ".")
			changePath := numbersToWildcardRegex.ReplaceAllString(joinedPath, ".*")

			if changePath == rule.Path {
				if rule.Reducers == nil {
					continue
				}

				for i := range *rule.Reducers {
					(*rule.Reducers)[i].To = d.To
					(*rule.Reducers)[i].From = d.From
				}

				filteredRules = append(filteredRules, rule)
			}
		}
	}

	return filteredRules
}

func (b *BaseExtractor) UnsupportedReducerRulesByDiffs(rules []Rule, ds diff.Changelog) []Rule {
	filteredRules := make([]Rule, 0)

	for _, rule := range b.ReducerRulesByDiffs(rules, ds) {
		if rule.Unsupported == nil {
			continue
		}

		if len(*rule.Unsupported) == 0 {
			continue
		}

		filteredRules = append(filteredRules, rule)
	}

	return filteredRules
}

func (b *BaseExtractor) UnsafeReducerRulesByDiffs(rules []Rule, ds diff.Changelog) []Rule {
	filteredRules := make([]Rule, 0)

	for _, rule := range b.ReducerRulesByDiffs(rules, ds) {
		if rule.Safe != nil && len(*rule.Safe) > 0 {
			if b.areReducersSafe(rule.Reducers, rule.Safe, ds) {
				continue
			}
		}

		filteredRules = append(filteredRules, rule)
	}

	return filteredRules
}

func (b *BaseExtractor) areReducersSafe(reducers *[]Reducer, safe *[]Safe, ds diff.Changelog) bool {
	if safe == nil {
		return false
	}

	for _, r := range *reducers {
		if !b.isReducerSafe(r, *safe, ds) {
			return false
		}
	}

	return true
}

func (*BaseExtractor) isReducerSafe(reducer Reducer, safe []Safe, ds diff.Changelog) bool {
	for _, s := range safe {
		// Check From/To conditions.
		fromToMatch := (s.From == nil || reducer.From == *s.From) && (s.To == nil || reducer.To == *s.To)

		// Check FromNodes conditions if present.
		fromNodesMatch := false

		if s.FromNodes != nil && len(*s.FromNodes) > 0 {
			allNodesMatch := true

			for _, node := range *s.FromNodes {
				if node.Path == nil {
					continue
				}

				// Check if the path exists in the diffs and has the expected value.
				nodeMatches := false

				for _, d := range ds {
					joinedPath := "." + strings.Join(d.Path, ".")
					if joinedPath == *node.Path &&
						((node.Value == nil && d.From == "none") ||
							(node.Value != nil && d.From == *node.Value)) {
						nodeMatches = true
						break
					}
				}

				if !nodeMatches {
					allNodesMatch = false

					break
				}
			}

			fromNodesMatch = allNodesMatch
		} else {
			// If no FromNodes conditions, consider it a match.
			fromNodesMatch = true
		}

		// If either From/To conditions or FromNodes conditions match, the rule is safe.
		if (s.FromNodes == nil && fromToMatch) ||
			(s.From == nil && s.To == nil && fromNodesMatch) ||
			(fromToMatch && fromNodesMatch) {
			return true
		}
	}

	return false
}

func (*BaseExtractor) ExtractImmutablesFromRules(rls []Rule) []string {
	immutables := make([]string, 0)

	for _, rule := range rls {
		if rule.Immutable {
			immutables = append(immutables, rule.Path)
		}
	}

	return immutables
}

func (*BaseExtractor) ExtractReducerRules(rls []Rule) []Rule {
	reducers := make([]Rule, 0)

	for _, rule := range rls {
		if rule.Reducers != nil {
			reducers = append(reducers, rule)
		}
	}

	return reducers
}
