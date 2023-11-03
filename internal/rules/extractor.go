// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rules

import (
	"regexp"
	"strings"

	"github.com/r3labs/diff/v3"
	"github.com/sirupsen/logrus"
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
	Reducers    *[]Reducer     `yaml:"reducers,omitempty"`
}

type Unsupported struct {
	From   *string `yaml:"from,omitempty"`
	To     *string `yaml:"to,omitempty"`
	Reason *string `yaml:"reason,omitempty"`
}

type Reducer struct {
	Key       string `yaml:"key"`
	Lifecycle string `yaml:"lifecycle"`
	From      string `yaml:"from"`
	To        string `yaml:"to"`
}

type Extractor interface {
	GetImmutables(phase string) []string
	GetReducers(phase string) []Rule
	ReducerRulesByDiffs(reducers []Rule, ds diff.Changelog) []Rule
	UnsupportedReducerRulesByDiffs(rules []Rule, ds diff.Changelog) []Rule
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
	filteredReducers := make([]Rule, 0)

	for _, rule := range rules {
		for _, d := range ds {
			joinedPath := "." + strings.Join(d.Path, ".")
			changePath := numbersToWildcardRegex.ReplaceAllString(joinedPath, ".*")

			if changePath == rule.Path {
				if rule.Reducers == nil {
					continue
				}

				toStr, toOk := d.To.(string)

				fromStr, fromOk := d.From.(string)

				if !fromOk || !toOk {
					logrus.Debugf("skipping reducer rule %s, from or to are not strings", rule.Path)

					continue
				}

				for i := range *rule.Reducers {
					(*rule.Reducers)[i].To = toStr
					(*rule.Reducers)[i].From = fromStr
				}

				filteredReducers = append(filteredReducers, rule)
			}
		}
	}

	return filteredReducers
}

func (b *BaseExtractor) UnsupportedReducerRulesByDiffs(rules []Rule, ds diff.Changelog) []Rule {
	filteredReducers := make([]Rule, 0)

	for _, rule := range b.ReducerRulesByDiffs(rules, ds) {
		if rule.Unsupported == nil {
			continue
		}

		if len(*rule.Unsupported) == 0 {
			continue
		}

		filteredReducers = append(filteredReducers, rule)
	}

	return filteredReducers
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
