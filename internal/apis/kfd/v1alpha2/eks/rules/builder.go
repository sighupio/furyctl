// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rules

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/r3labs/diff/v3"
	"github.com/sirupsen/logrus"

	"github.com/sighupio/furyctl/internal/diffs"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

var ErrReadingRulesFile = errors.New("error while reading rules file")

type EKSRulesSpec struct {
	Infrastructure []Rule `yaml:"infrastructure"`
	Kubernetes     []Rule `yaml:"kubernetes"`
	Distribution   []Rule `yaml:"distribution"`
}

type Rule struct {
	Path        string     `yaml:"path"`
	Immutable   bool       `yaml:"immutable"`
	Description *string    `yaml:"description"`
	Reducers    *[]Reducer `yaml:"reducers"`
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
}

type EKSExtractor struct {
	Spec EKSRulesSpec
}

func NewEKSClusterRulesExtractor(distributionPath string) (*EKSExtractor, error) {
	builder := EKSExtractor{}

	rulesPath := filepath.Join(distributionPath, "rules", "ekscluster-kfd-v1alpha2.yaml")

	spec, err := yamlx.FromFileV3[EKSRulesSpec](rulesPath)
	if err != nil {
		return &builder, fmt.Errorf("%w: %s", ErrReadingRulesFile, err)
	}

	builder.Spec = spec

	return &builder, nil
}

func (r *EKSExtractor) GetImmutables(phase string) []string {
	switch phase {
	case "infrastructure":
		return r.extractImmutablesFromRules(r.Spec.Infrastructure)

	case "kubernetes":
		return r.extractImmutablesFromRules(r.Spec.Kubernetes)

	case "distribution":
		return r.extractImmutablesFromRules(r.Spec.Distribution)

	default:
		return []string{}
	}
}

func (r *EKSExtractor) GetReducers(phase string) []Rule {
	switch phase {
	case "infrastructure":
		return r.extractReducerRules(r.Spec.Infrastructure)

	case "kubernetes":
		return r.extractReducerRules(r.Spec.Kubernetes)

	case "distribution":
		return r.extractReducerRules(r.Spec.Distribution)

	default:
		return []Rule{}
	}
}

func (*EKSExtractor) ReducerRulesByDiffs(rules []Rule, ds diff.Changelog) []Rule {
	filteredReducers := make([]Rule, 0)

	for _, rule := range rules {
		for _, d := range ds {
			joinedPath := "." + strings.Join(d.Path, ".")
			changePath := diffs.NumbersToWildcardRegex.ReplaceAllString(joinedPath, ".*")

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

func (*EKSExtractor) extractImmutablesFromRules(rules []Rule) []string {
	immutables := make([]string, 0)

	for _, rule := range rules {
		if rule.Immutable {
			immutables = append(immutables, rule.Path)
		}
	}

	return immutables
}

func (*EKSExtractor) extractReducerRules(rules []Rule) []Rule {
	reducers := make([]Rule, 0)

	for _, rule := range rules {
		if rule.Reducers != nil {
			reducers = append(reducers, rule)
		}
	}

	return reducers
}
