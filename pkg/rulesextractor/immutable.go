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

type ImmutableExtractor struct {
	*BaseExtractor

	Spec Spec
}

func NewImmutableClusterRulesExtractor(
	distributionPath string,
	renderedConfig map[string]any,
) (*ImmutableExtractor, error) {
	builder := ImmutableExtractor{
		BaseExtractor: &BaseExtractor{
			RenderedConfig: renderedConfig,
		},
	}

	rulesPath := filepath.Join(distributionPath, "rules", "immutable-kfd-v1alpha2.yaml")

	spec, err := yamlx.FromFileV3[Spec](rulesPath)
	if err != nil {
		return &builder, fmt.Errorf("%w: %s", ErrReadingRulesFile, err)
	}

	builder.Spec = spec
	builder.BaseExtractor = NewBaseExtractor(spec)
	builder.RenderedConfig = renderedConfig

	return &builder, nil
}

func (r *ImmutableExtractor) GetImmutableRules(phase string) []Rule {
	switch phase {
	case cluster.OperationPhaseInfrastructure,
		cluster.OperationPhaseKubernetes,
		cluster.OperationPhaseDistribution:
		return extractFromPhase(r.Spec, phase, r.ExtractImmutableRules)
	default:
		return []Rule{}
	}
}

func (r *ImmutableExtractor) GetReducers(phase string) []Rule {
	switch phase {
	case cluster.OperationPhaseInfrastructure,
		cluster.OperationPhaseKubernetes,
		cluster.OperationPhaseDistribution:
		return extractFromPhase(r.Spec, phase, r.ExtractReducerRules)
	default:
		return []Rule{}
	}
}

func (r *ImmutableExtractor) GetUnsupportedRules(phase string) []Rule {
	switch phase {
	case cluster.OperationPhaseInfrastructure,
		cluster.OperationPhaseKubernetes,
		cluster.OperationPhaseDistribution:
		return extractFromPhase(r.Spec, phase, r.ExtractUnsupportedRules)
	default:
		return []Rule{}
	}
}

func (r *ImmutableExtractor) ReducerRulesByDiffs(rls []Rule, ds diff.Changelog) []Rule {
	return r.BaseExtractor.ReducerRulesByDiffs(rls, ds)
}

func (r *ImmutableExtractor) UnsupportedReducerRulesByDiffs(rls []Rule, ds diff.Changelog) []Rule {
	return r.BaseExtractor.UnsupportedReducerRulesByDiffs(rls, ds)
}

func (r *ImmutableExtractor) UnsafeReducerRulesByDiffs(rls []Rule, ds diff.Changelog) []Rule {
	return r.BaseExtractor.UnsafeReducerRulesByDiffs(rls, ds)
}

func (r *ImmutableExtractor) FilterSafeImmutableRules(rules []Rule, ds diff.Changelog) []Rule {
	return r.BaseExtractor.FilterSafeImmutableRules(rules, ds)
}
