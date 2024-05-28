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

type OnPremExtractor struct {
	*BaseExtractor
	Spec Spec
}

func NewOnPremClusterRulesExtractor(distributionPath string) (*OnPremExtractor, error) {
	builder := OnPremExtractor{}

	rulesPath := filepath.Join(distributionPath, "rules", "onpremises-kfd-v1alpha2.yaml")

	spec, err := yamlx.FromFileV3[Spec](rulesPath)
	if err != nil {
		return &builder, fmt.Errorf("%w: %s", ErrReadingRulesFile, err)
	}

	builder.Spec = spec

	return &builder, nil
}

func (r *OnPremExtractor) GetImmutables(phase string) []string {
	switch phase {
	case cluster.OperationPhaseKubernetes:
		if r.Spec.Kubernetes == nil {
			return []string{}
		}

		return r.BaseExtractor.ExtractImmutablesFromRules(*r.Spec.Kubernetes)

	case cluster.OperationPhaseDistribution:
		if r.Spec.Distribution == nil {
			return []string{}
		}

		return r.BaseExtractor.ExtractImmutablesFromRules(*r.Spec.Distribution)

	default:
		return []string{}
	}
}

func (r *OnPremExtractor) GetReducers(phase string) []Rule {
	switch phase {
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

func (r *OnPremExtractor) ReducerRulesByDiffs(rls []Rule, ds diff.Changelog) []Rule {
	return r.BaseExtractor.ReducerRulesByDiffs(rls, ds)
}

func (r *OnPremExtractor) UnsupportedReducerRulesByDiffs(rls []Rule, ds diff.Changelog) []Rule {
	return r.BaseExtractor.UnsupportedReducerRulesByDiffs(rls, ds)
}

func (r *OnPremExtractor) UnsafeReducerRulesByDiffs(rls []Rule, ds diff.Changelog) []Rule {
	return r.BaseExtractor.UnsafeReducerRulesByDiffs(rls, ds)
}
