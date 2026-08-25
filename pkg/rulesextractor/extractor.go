// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rulesextractor

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/r3labs/diff/v3"
	"github.com/samber/lo"
	"github.com/sirupsen/logrus"

	"github.com/sighupio/furyctl/internal/cluster"
)

var numbersToWildcardRegex = regexp.MustCompile(`\.\d+\b`)

// pathToRegex converts a path pattern with wildcard (**) into a regex pattern.
// ** matches zero or more path segments (recursive).
func pathToRegex(path string) string {
	// Escape special regex characters except for ** which we'll handle.
	escaped := regexp.QuoteMeta(path)

	// Replace escaped \*\* with a placeholder (__DOUBLE_STAR__) to preserve it during processing.
	// This placeholder allows us to distinguish between different contexts where ** appears:
	// - \.__DOUBLE_STAR__\. (with dots on both sides): should match zero or more segments between dots
	// - __DOUBLE_STAR__ at other positions (start, end, or without surrounding dots): should match any characters
	// Without the placeholder, we couldn't tell these cases apart when doing replacements.
	escaped = strings.ReplaceAll(escaped, "\\*\\*", "__DOUBLE_STAR__")

	// Handle ** surrounded by dots: replaces the pattern with regex that allows zero or more segments between them.
	escaped = strings.ReplaceAll(escaped, "\\.__DOUBLE_STAR__\\.", "(?:\\..*)?\\.")

	// Replace remaining __DOUBLE_STAR__ (at start, end, or without surrounding dots) with .* to match any characters.
	escaped = strings.ReplaceAll(escaped, "__DOUBLE_STAR__", ".*")

	// Anchor the pattern to match the entire string.
	return "^" + escaped + "$"
}

// MatchesPattern checks if a given path matches a pattern that may contain wildcards.
func MatchesPattern(path, pattern string) bool {
	regexPattern := pathToRegex(pattern)

	regex, err := regexp.Compile(regexPattern)
	if err != nil {
		return false
	}

	return regex.MatchString(path)
}

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

// Paths returns the paths of the given rules.
func Paths(rls []Rule) []string {
	return lo.Map(rls, func(rule Rule, _ int) string {
		return rule.Path
	})
}

type Unsupported struct {
	From   *any    `yaml:"from,omitempty"`
	To     *any    `yaml:"to,omitempty"`
	Reason *string `yaml:"reason,omitempty"`
}

type FromNode struct {
	Path *string `yaml:"path"`
	From *string `yaml:"from"`
	To   *string `yaml:"to"`
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
	GetUnsupportedRules(phase string) []Rule
	ReducerRulesByDiffs(reducers []Rule, ds diff.Changelog) []Rule
	UnsupportedReducerRulesByDiffs(rules []Rule, ds diff.Changelog) []Rule
	UnsafeReducerRulesByDiffs(rules []Rule, ds diff.Changelog) []Rule
}

type BaseExtractor struct {
	Spec            Spec
	RenderedConfig  map[string]any
	SupportedPhases cluster.SupportedPhases
}

type PathNotFoundError struct {
	Key string
}

func (e *PathNotFoundError) Error() string {
	return fmt.Sprintf("key '%s' not found in path", e.Key)
}

type NotAMapError struct {
	Key string
}

func (e *NotAMapError) Error() string {
	return fmt.Sprintf("path element '%s' is not a map", e.Key)
}

func NewBaseExtractor(spec Spec) *BaseExtractor {
	return &BaseExtractor{
		Spec: spec,
	}
}

// phaseRules returns the rule slice for the given operation phase, or nil.
func phaseRules(spec Spec, phase string) *[]Rule {
	switch phase {
	case cluster.OperationPhaseInfrastructure:
		return spec.Infrastructure
	case cluster.OperationPhaseKubernetes:
		return spec.Kubernetes
	case cluster.OperationPhaseDistribution:
		return spec.Distribution
	default:
		return nil
	}
}

// extractFromPhase applies extract to the rules of the given phase.
func extractFromPhase(spec Spec, phase string, extract func(*[]Rule) []Rule) []Rule {
	return extract(phaseRules(spec, phase))
}

func (b *BaseExtractor) GetImmutables(_ string) []string {
	return slices.Concat(
		b.ExtractImmutablesFromRules(b.Spec.Infrastructure),
		b.ExtractImmutablesFromRules(b.Spec.Kubernetes),
		b.ExtractImmutablesFromRules(b.Spec.Distribution),
	)
}

func (b *BaseExtractor) GetImmutableRules(phase string) []Rule {
	if !b.supportsPhase(phase) {
		return []Rule{}
	}

	return extractFromPhase(b.Spec, phase, b.ExtractImmutableRules)
}

func (b *BaseExtractor) FilterSafeImmutableRules(rules []Rule, ds diff.Changelog) []Rule {
	// Drop the rules whose safe conditions match.
	return lo.Reject(rules, func(rule Rule, _ int) bool {
		return rule.Safe != nil && len(*rule.Safe) > 0 && b.isImmutableRuleSafe(rule, ds)
	})
}

func (b *BaseExtractor) GetReducers(phase string) []Rule {
	if !b.supportsPhase(phase) {
		return []Rule{}
	}

	return extractFromPhase(b.Spec, phase, b.ExtractReducerRules)
}

func (b *BaseExtractor) GetUnsupportedRules(phase string) []Rule {
	if !b.supportsPhase(phase) {
		return []Rule{}
	}

	return extractFromPhase(b.Spec, phase, b.ExtractUnsupportedRules)
}

func (*BaseExtractor) ReducerRulesByDiffs(rules []Rule, ds diff.Changelog) []Rule {
	filteredRules := make([]Rule, 0)

	for _, rule := range rules {
		for _, d := range ds {
			joinedPath := "." + strings.Join(d.Path, ".")
			changePath := numbersToWildcardRegex.ReplaceAllString(joinedPath, ".*")

			if MatchesPattern(changePath, rule.Path) && rule.Reducers != nil {
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
	return lo.Filter(b.ReducerRulesByDiffs(rules, ds), func(rule Rule, _ int) bool {
		return rule.Unsupported != nil && len(*rule.Unsupported) > 0
	})
}

func (b *BaseExtractor) UnsafeReducerRulesByDiffs(rules []Rule, ds diff.Changelog) []Rule {
	return lo.Reject(b.ReducerRulesByDiffs(rules, ds), func(rule Rule, _ int) bool {
		return rule.Safe != nil && len(*rule.Safe) > 0 && b.areReducersSafe(rule.Reducers, rule.Safe, ds)
	})
}

// filterRules returns the rules matching the keep predicate, or an empty slice
// if rules is nil.
func filterRules(rules *[]Rule, keep func(Rule) bool) []Rule {
	return lo.Filter(lo.FromPtr(rules), func(rule Rule, _ int) bool {
		return keep(rule)
	})
}

func (b *BaseExtractor) ExtractImmutablesFromRules(rules *[]Rule) []string {
	return Paths(b.ExtractImmutableRules(rules))
}

func (*BaseExtractor) ExtractImmutableRules(rules *[]Rule) []Rule {
	return filterRules(rules, func(rule Rule) bool {
		return rule.Immutable
	})
}

func (*BaseExtractor) ExtractReducerRules(rules *[]Rule) []Rule {
	return filterRules(rules, func(rule Rule) bool {
		return rule.Reducers != nil
	})
}

// ExtractUnsupportedRules returns the rules that declare at least one unsupported
// transition, regardless of whether they also define reducers. Unsupported
// transitions are enforced independently from reducers (see
// diffs.AssertReducerUnsupportedViolations).
func (*BaseExtractor) ExtractUnsupportedRules(rules *[]Rule) []Rule {
	return filterRules(rules, func(rule Rule) bool {
		return rule.Unsupported != nil && len(*rule.Unsupported) > 0
	})
}

// supportsPhase reports whether the given phase is enabled for this extractor.
// A nil SupportedPhases list means no restriction.
func (b *BaseExtractor) supportsPhase(phase string) bool {
	return b.SupportedPhases == nil || b.SupportedPhases.IsSupported(phase)
}

func (b *BaseExtractor) isImmutableRuleSafe(rule Rule, ds diff.Changelog) bool {
	if rule.Safe == nil || len(*rule.Safe) == 0 {
		return false
	}

	// Find the diff that matches this rule's path; the zero Change leaves both
	// From and To nil, as when no diff matches.
	matchingDiff, _ := lo.Find(ds, func(d diff.Change) bool {
		joinedPath := "." + strings.Join(d.Path, ".")
		changePath := numbersToWildcardRegex.ReplaceAllString(joinedPath, ".*")

		return MatchesPattern(changePath, rule.Path)
	})

	return lo.SomeBy(*rule.Safe, func(s Safe) bool {
		// Check From/To conditions.
		fromToMatch := (s.From == nil || matchingDiff.From == *s.From) &&
			(s.To == nil || matchingDiff.To == *s.To)

		// Check FromNodes conditions.
		fromNodesMatch := b.areNodeConditionsMet(s.FromNodes, ds)

		// If either From/To conditions or FromNodes conditions match, the rule is safe.
		return (s.FromNodes == nil && fromToMatch) ||
			(s.From == nil && s.To == nil && fromNodesMatch) ||
			(fromToMatch && fromNodesMatch)
	})
}

func (b *BaseExtractor) areNodeConditionsMet(fromNodes *[]FromNode, ds diff.Changelog) bool {
	if fromNodes == nil || len(*fromNodes) == 0 {
		return true // No conditions means they're met by default.
	}

	// We need at least one node to match.
	return lo.SomeBy(*fromNodes, func(node FromNode) bool {
		if node.Path == nil {
			return false
		}

		isNodePath := func(d diff.Change) bool {
			return "."+strings.Join(d.Path, ".") == *node.Path
		}

		// Check if the path exists in the diffs and has the expected value.
		if lo.ContainsBy(ds, func(d diff.Change) bool {
			return isNodePath(d) &&
				b.checkConditionFrom(node.From, d.From) &&
				b.checkConditionTo(node.To, d.To)
		}) {
			return true
		}

		// The path did change but with an unexpected value: not a match.
		if lo.ContainsBy(ds, isNodePath) {
			return false
		}

		// The path is not in the diffs at all: compare against its unchanged value.
		unchangedValue, err := getNestedValue(b.RenderedConfig, *node.Path)
		if err != nil {
			logrus.Error(fmt.Sprintf("error getting value for %s: %s", *node.Path, err))

			return false
		}

		return b.checkConditionFrom(node.From, unchangedValue) &&
			b.checkConditionTo(node.To, unchangedValue)
	})
}

func getNestedValue(m map[string]any, path string) (any, error) {
	// Remove leading dot if present.
	path = strings.TrimPrefix(path, ".")

	// Split the path into individual keys.
	keys := strings.Split(path, ".")

	// Start with the root map.
	current := any(m)

	// Traverse the nested structure.
	for _, key := range keys {
		// Skip empty keys.
		if key == "" {
			continue
		}

		// Check if current is a map.
		currentMap, ok := current.(map[string]any)
		if !ok {
			return nil, &NotAMapError{Key: key}
		}

		// Look for the key in the current map.
		value, exists := currentMap[key]
		if !exists {
			return nil, &PathNotFoundError{Key: key}
		}

		// Move to the next level.
		current = value
	}

	return current, nil
}

func (*BaseExtractor) checkConditionFrom(nodeFrom *string, diffFrom any) bool {
	if nodeFrom == nil || *nodeFrom == "" {
		return true
	}

	return (*nodeFrom == "none" && diffFrom == nil) || (diffFrom != nil && diffFrom == *nodeFrom)
}

func (*BaseExtractor) checkConditionTo(nodeTo *string, diffTo any) bool {
	if nodeTo == nil || *nodeTo == "" {
		return true
	}

	return (*nodeTo == "none" && diffTo == nil) || (diffTo != nil && diffTo == *nodeTo)
}

func (b *BaseExtractor) areReducersSafe(reducers *[]Reducer, safe *[]Safe, ds diff.Changelog) bool {
	if safe == nil {
		return false
	}

	return lo.EveryBy(*reducers, func(r Reducer) bool {
		return b.isReducerSafe(r, *safe, ds)
	})
}

func (b *BaseExtractor) isReducerSafe(reducer Reducer, safe []Safe, ds diff.Changelog) bool {
	return lo.SomeBy(safe, func(s Safe) bool {
		// Check From/To conditions.
		fromToMatch := (s.From == nil || reducer.From == *s.From) && (s.To == nil || reducer.To == *s.To)

		// Check FromNodes conditions using the dedicated function.
		fromNodesMatch := b.areNodeConditionsMet(s.FromNodes, ds)

		// If either From/To conditions or FromNodes conditions match, the rule is safe.
		return (s.FromNodes == nil && fromToMatch) ||
			(s.From == nil && s.To == nil && fromNodesMatch) ||
			(fromToMatch && fromNodesMatch)
	})
}
