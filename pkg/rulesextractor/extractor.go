// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rulesextractor

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/r3labs/diff/v3"
	"github.com/sirupsen/logrus"
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
	Spec           Spec
	RenderedConfig map[string]any
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
		// If the rule has safe conditions and they match, skip this rule.
		if rule.Safe != nil && len(*rule.Safe) > 0 && b.isImmutableRuleSafe(rule, ds) {
			continue
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

// GetUnsupportedRules returns all the rules that declare unsupported transitions,
// regardless of whether they also define reducers. This is what drives the
// "unsupported transition" validation, which must not depend on reducers being set.
func (b *BaseExtractor) GetUnsupportedRules(_ string) []Rule {
	var unsupported []Rule

	if b.Spec.Infrastructure != nil {
		unsupported = append(unsupported, b.ExtractUnsupportedRules(*b.Spec.Infrastructure)...)
	}

	if b.Spec.Kubernetes != nil {
		unsupported = append(unsupported, b.ExtractUnsupportedRules(*b.Spec.Kubernetes)...)
	}

	if b.Spec.Distribution != nil {
		unsupported = append(unsupported, b.ExtractUnsupportedRules(*b.Spec.Distribution)...)
	}

	return unsupported
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
	filteredRules := make([]Rule, 0)

	for _, rule := range b.ReducerRulesByDiffs(rules, ds) {
		if rule.Unsupported == nil || len(*rule.Unsupported) == 0 {
			continue
		}

		filteredRules = append(filteredRules, rule)
	}

	return filteredRules
}

func (b *BaseExtractor) UnsafeReducerRulesByDiffs(rules []Rule, ds diff.Changelog) []Rule {
	filteredRules := make([]Rule, 0)

	for _, rule := range b.ReducerRulesByDiffs(rules, ds) {
		if rule.Safe != nil && len(*rule.Safe) > 0 && b.areReducersSafe(rule.Reducers, rule.Safe, ds) {
			continue
		}

		filteredRules = append(filteredRules, rule)
	}

	return filteredRules
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

// ExtractUnsupportedRules returns the rules that declare at least one unsupported
// transition, regardless of whether they also define reducers. Unsupported
// transitions are enforced independently from reducers (see
// diffs.AssertReducerUnsupportedViolations).
func (*BaseExtractor) ExtractUnsupportedRules(rls []Rule) []Rule {
	unsupported := make([]Rule, 0)

	for _, rule := range rls {
		if rule.Unsupported != nil && len(*rule.Unsupported) > 0 {
			unsupported = append(unsupported, rule)
		}
	}

	return unsupported
}

func (b *BaseExtractor) isImmutableRuleSafe(rule Rule, ds diff.Changelog) bool {
	if rule.Safe == nil || len(*rule.Safe) == 0 {
		return false
	}

	// Find the diff that matches this rule's path.
	var matchingDiffFrom, matchingDiffTo any

	for _, d := range ds {
		joinedPath := "." + strings.Join(d.Path, ".")
		changePath := numbersToWildcardRegex.ReplaceAllString(joinedPath, ".*")

		if MatchesPattern(changePath, rule.Path) {
			matchingDiffFrom = d.From
			matchingDiffTo = d.To

			break
		}
	}

	for _, s := range *rule.Safe {
		// Check From/To conditions.
		fromToMatch := (s.From == nil || matchingDiffFrom == *s.From) &&
			(s.To == nil || matchingDiffTo == *s.To)

		// Check FromNodes conditions.
		fromNodesMatch := b.areNodeConditionsMet(s.FromNodes, ds)

		// If either From/To conditions or FromNodes conditions match, the rule is safe.
		if (s.FromNodes == nil && fromToMatch) ||
			(s.From == nil && s.To == nil && fromNodesMatch) ||
			(fromToMatch && fromNodesMatch) {
			return true
		}
	}

	return false
}

func (b *BaseExtractor) areNodeConditionsMet(fromNodes *[]FromNode, ds diff.Changelog) bool {
	if fromNodes == nil || len(*fromNodes) == 0 {
		return true // No conditions means they're met by default.
	}

	// We need at least one node to match.
	anyNodeMatches := false

	for _, node := range *fromNodes {
		if node.Path == nil {
			continue
		}

		// Check if the path exists in the diffs and has the expected value.
		nodeMatches := false
		foundNodeInDiff := false

		for _, d := range ds {
			joinedPath := "." + strings.Join(d.Path, ".")
			if joinedPath == *node.Path {
				foundNodeInDiff = true
				// Check if the node matches based on From/To or Value.
				nodeMatches = b.checkConditionFrom(node.From, d.From) &&
					b.checkConditionTo(node.To, d.To)

				if nodeMatches {
					break
				}
			}
		}

		if !foundNodeInDiff && !nodeMatches {
			unchangedValue, err := getNestedValue(b.RenderedConfig, *node.Path)
			if err == nil {
				nodeMatches = b.checkConditionFrom(node.From, unchangedValue) &&
					b.checkConditionTo(node.To, unchangedValue)
			} else {
				logrus.Error(fmt.Sprintf("error getting value for %s: %s", *node.Path, err))
			}
		}

		if nodeMatches {
			anyNodeMatches = true

			break // We found a matching node, no need to check others.
		}
	}

	return anyNodeMatches
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

	for _, r := range *reducers {
		if !b.isReducerSafe(r, *safe, ds) {
			return false
		}
	}

	return true
}

func (b *BaseExtractor) isReducerSafe(reducer Reducer, safe []Safe, ds diff.Changelog) bool {
	for _, s := range safe {
		// Check From/To conditions.
		fromToMatch := (s.From == nil || reducer.From == *s.From) && (s.To == nil || reducer.To == *s.To)

		// Check FromNodes conditions using the dedicated function.
		fromNodesMatch := b.areNodeConditionsMet(s.FromNodes, ds)

		// If either From/To conditions or FromNodes conditions match, the rule is safe.
		if (s.FromNodes == nil && fromToMatch) ||
			(s.From == nil && s.To == nil && fromNodesMatch) ||
			(fromToMatch && fromNodesMatch) {
			return true
		}
	}

	return false
}
