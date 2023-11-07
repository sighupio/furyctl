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

type DistroExtractor struct {
	*rules.BaseExtractor
	Spec rules.Spec
}

func NewDistroClusterRulesExtractor(distributionPath string) (*DistroExtractor, error) {
	builder := DistroExtractor{}

	rulesPath := filepath.Join(distributionPath, "rules", "kfddistribution-kfd-v1alpha2.yaml")

	spec, err := yamlx.FromFileV3[rules.Spec](rulesPath)
	if err != nil {
		return &builder, fmt.Errorf("%w: %s", ErrReadingRulesFile, err)
	}

	builder.Spec = spec

	return &builder, nil
}

func (r *DistroExtractor) GetImmutables(phase string) []string {
	switch phase {
	case "distribution":
		if r.Spec.Distribution == nil {
			return []string{}
		}

		return r.BaseExtractor.ExtractImmutablesFromRules(*r.Spec.Distribution)

	default:
		return []string{}
	}
}

func (r *DistroExtractor) GetReducers(phase string) []rules.Rule {
	switch phase {
	case "distribution":
		if r.Spec.Distribution == nil {
			return []rules.Rule{}
		}

		return r.BaseExtractor.ExtractReducerRules(*r.Spec.Distribution)

	default:
		return []rules.Rule{}
	}
}

func (r *DistroExtractor) ReducerRulesByDiffs(rls []rules.Rule, ds diff.Changelog) []rules.Rule {
	return r.BaseExtractor.ReducerRulesByDiffs(rls, ds)
}

func (r *DistroExtractor) UnsupportedReducerRulesByDiffs(rls []rules.Rule, ds diff.Changelog) []rules.Rule {
	return r.BaseExtractor.UnsupportedReducerRulesByDiffs(rls, ds)
}
