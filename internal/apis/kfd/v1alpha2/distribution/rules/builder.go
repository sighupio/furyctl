// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rules

import (
	"errors"
	"fmt"
	"path/filepath"

	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

var ErrReadingRulesFile = errors.New("error while reading rules file")

type DistroRulesSpec struct {
	Distribution []Rule `yaml:"distribution"`
}

type Rule struct {
	Path      string `yaml:"path"`
	Immutable bool   `yaml:"immutable"`
}

type Extractor interface {
	GetImmutables(phase string) []string
}

type DistroExtractor struct {
	Spec DistroRulesSpec
}

func NewDistroClusterRulesExtractor(distributionPath string) (*DistroExtractor, error) {
	builder := DistroExtractor{}

	rulesPath := filepath.Join(distributionPath, "rules", "kfddistribution-kfd-v1alpha2.yaml")

	spec, err := yamlx.FromFileV3[DistroRulesSpec](rulesPath)
	if err != nil {
		return &builder, fmt.Errorf("%w: %s", ErrReadingRulesFile, err)
	}

	builder.Spec = spec

	return &builder, nil
}

func (r *DistroExtractor) GetImmutables(phase string) []string {
	switch phase {
	case "distribution":
		return r.extractImmutablesFromRules(r.Spec.Distribution)

	default:
		return []string{}
	}
}

func (*DistroExtractor) extractImmutablesFromRules(rules []Rule) []string {
	var immutables []string

	for _, rule := range rules {
		if rule.Immutable {
			immutables = append(immutables, rule.Path)
		}
	}

	return immutables
}
