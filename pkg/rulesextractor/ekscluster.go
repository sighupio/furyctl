// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rulesextractor

import (
	"fmt"
	"path/filepath"

	"github.com/r3labs/diff/v3"

	"github.com/sighupio/furyctl/internal/cluster"
	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

type EKSExtractor struct {
	*BaseExtractor
	Spec Spec
}

func NewEKSClusterRulesExtractor(distributionPath string, renderedConfig map[string]any) (*EKSExtractor, error) {
	builder := EKSExtractor{}

	rulesPath := filepath.Join(distributionPath, "rules", "ekscluster-kfd-v1alpha2.yaml")

	spec, err := yamlx.FromFileV3[Spec](rulesPath)
	if err != nil {
		return &builder, fmt.Errorf("%w: %s", ErrReadingRulesFile, err)
	}

	builder.Spec = spec
	builder.BaseExtractor = NewBaseExtractor(spec)
	builder.BaseExtractor.RenderedConfig = renderedConfig

	return &builder, nil
}

func (r *EKSExtractor) GetImmutableRules(phase string) []Rule {
	switch phase {
	case cluster.OperationPhaseInfrastructure:
		if r.Spec.Infrastructure == nil {
			return []Rule{}
		}

		var immutableRules []Rule

		for _, rule := range *r.Spec.Infrastructure {
			if rule.Immutable {
				immutableRules = append(immutableRules, rule)
			}
		}

		return immutableRules

	case cluster.OperationPhaseKubernetes:
		if r.Spec.Kubernetes == nil {
			return []Rule{}
		}

		var immutableRules []Rule

		for _, rule := range *r.Spec.Kubernetes {
			if rule.Immutable {
				immutableRules = append(immutableRules, rule)
			}
		}

		return immutableRules

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

func (r *EKSExtractor) GetReducers(phase string) []Rule {
	switch phase {
	case cluster.OperationPhaseInfrastructure:
		if r.Spec.Infrastructure == nil {
			return []Rule{}
		}

		return r.BaseExtractor.ExtractReducerRules(*r.Spec.Infrastructure)

	case cluster.OperationPhaseKubernetes:
		if r.Spec.Kubernetes == nil {
			return []Rule{}
		}

		return r.BaseExtractor.ExtractReducerRules(*r.Spec.Kubernetes)

	case cluster.OperationPhaseDistribution:
		if r.Spec.Distribution == nil {
			return []Rule{}
		}

		return r.BaseExtractor.ExtractReducerRules(*r.Spec.Distribution)

	default:
		return []Rule{}
	}
}

func (r *EKSExtractor) ReducerRulesByDiffs(rls []Rule, ds diff.Changelog) []Rule {
	return r.BaseExtractor.ReducerRulesByDiffs(rls, ds)
}

func (r *EKSExtractor) UnsupportedReducerRulesByDiffs(rls []Rule, ds diff.Changelog) []Rule {
	return r.BaseExtractor.UnsupportedReducerRulesByDiffs(rls, ds)
}

func (r *EKSExtractor) UnsafeReducerRulesByDiffs(rls []Rule, ds diff.Changelog) []Rule {
	return r.BaseExtractor.UnsafeReducerRulesByDiffs(rls, ds)
}

func (r *EKSExtractor) FilterSafeImmutableRules(rules []Rule, ds diff.Changelog) []Rule {
	return r.BaseExtractor.FilterSafeImmutableRules(rules, ds)
}
