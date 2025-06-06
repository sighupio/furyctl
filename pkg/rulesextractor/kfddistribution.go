// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rulesextractor

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/r3labs/diff/v3"

	"github.com/sighupio/furyctl/internal/cluster"
	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

var ErrReadingRulesFile = errors.New("error while reading rules file")

type DistroExtractor struct {
	*BaseExtractor
	Spec Spec
}

func NewDistroClusterRulesExtractor(distributionPath string, renderedConfig map[string]any) (*DistroExtractor, error) {
	builder := DistroExtractor{
		BaseExtractor: &BaseExtractor{
			RenderedConfig: renderedConfig,
		},
	}

	rulesPath := filepath.Join(distributionPath, "rules", "kfddistribution-kfd-v1alpha2.yaml")

	spec, err := yamlx.FromFileV3[Spec](rulesPath)
	if err != nil {
		return &builder, fmt.Errorf("%w: %s", ErrReadingRulesFile, err)
	}

	builder.Spec = spec

	return &builder, nil
}

func (r *DistroExtractor) GetImmutableRules(phase string) []Rule {
	switch phase {
	case cluster.OperationPhaseDistribution:
		if r.Spec.Distribution == nil {
			return []Rule{}
		}

		var immutableRules []Rule

		for _, rule := range *r.Spec.Distribution {
			if rule.Immutable {
				immutableRules = append(immutableRules, rule)
			}
		}

		return immutableRules

	default:
		return []Rule{}
	}
}

func (r *DistroExtractor) GetReducers(phase string) []Rule {
	switch phase {
	case cluster.OperationPhaseDistribution:
		if r.Spec.Distribution == nil {
			return []Rule{}
		}

		return r.BaseExtractor.ExtractReducerRules(*r.Spec.Distribution)

	default:
		return []Rule{}
	}
}

func (r *DistroExtractor) ReducerRulesByDiffs(rls []Rule, ds diff.Changelog) []Rule {
	return r.BaseExtractor.ReducerRulesByDiffs(rls, ds)
}

func (r *DistroExtractor) UnsupportedReducerRulesByDiffs(rls []Rule, ds diff.Changelog) []Rule {
	return r.BaseExtractor.UnsupportedReducerRulesByDiffs(rls, ds)
}

func (r *DistroExtractor) UnsafeReducerRulesByDiffs(rls []Rule, ds diff.Changelog) []Rule {
	return r.BaseExtractor.UnsafeReducerRulesByDiffs(rls, ds)
}

func (r *DistroExtractor) FilterSafeImmutableRules(rules []Rule, ds diff.Changelog) []Rule {
	return r.BaseExtractor.FilterSafeImmutableRules(rules, ds)
}
