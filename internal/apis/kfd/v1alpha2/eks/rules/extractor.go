// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rules

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/r3labs/diff/v3"

	"github.com/sighupio/furyctl/internal/rules"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

var ErrReadingRulesFile = errors.New("error while reading rules file")

type EKSExtractor struct {
	*rules.BaseExtractor
	Spec rules.Spec
}

func NewEKSClusterRulesExtractor(distributionPath string) (*EKSExtractor, error) {
	builder := EKSExtractor{}

	rulesPath := filepath.Join(distributionPath, "rules", "ekscluster-kfd-v1alpha2.yaml")

	spec, err := yamlx.FromFileV3[rules.Spec](rulesPath)
	if err != nil {
		return &builder, fmt.Errorf("%w: %s", ErrReadingRulesFile, err)
	}

	builder.Spec = spec
	builder.BaseExtractor = rules.NewBaseExtractor(spec)

	return &builder, nil
}

func (r *EKSExtractor) GetImmutables(phase string) []string {
	switch phase {
	case "infrastructure":
		if r.Spec.Infrastructure == nil {
			return []string{}
		}

		return r.BaseExtractor.ExtractImmutablesFromRules(*r.Spec.Infrastructure)

	case "kubernetes":
		if r.Spec.Kubernetes == nil {
			return []string{}
		}

		return r.BaseExtractor.ExtractImmutablesFromRules(*r.Spec.Kubernetes)

	case "distribution":
		if r.Spec.Distribution == nil {
			return []string{}
		}

		return r.BaseExtractor.ExtractImmutablesFromRules(*r.Spec.Distribution)

	default:
		return []string{}
	}
}

func (r *EKSExtractor) GetReducers(phase string) []rules.Rule {
	switch phase {
	case "infrastructure":
		if r.Spec.Infrastructure == nil {
			return []rules.Rule{}
		}

		return r.BaseExtractor.ExtractReducerRules(*r.Spec.Infrastructure)

	case "kubernetes":
		if r.Spec.Kubernetes == nil {
			return []rules.Rule{}
		}

		return r.BaseExtractor.ExtractReducerRules(*r.Spec.Kubernetes)

	case "distribution":
		if r.Spec.Distribution == nil {
			return []rules.Rule{}
		}

		return r.BaseExtractor.ExtractReducerRules(*r.Spec.Distribution)

	default:
		return []rules.Rule{}
	}
}

func (r *EKSExtractor) ReducerRulesByDiffs(rls []rules.Rule, ds diff.Changelog) []rules.Rule {
	return r.BaseExtractor.ReducerRulesByDiffs(rls, ds)
}

func (r *EKSExtractor) UnsupportedReducerRulesByDiffs(rls []rules.Rule, ds diff.Changelog) []rules.Rule {
	return r.BaseExtractor.UnsupportedReducerRulesByDiffs(rls, ds)
}
