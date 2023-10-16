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

var errReadingRulesFile = errors.New("error while reading rules file")

type DistroRulesSpec struct {
	Distribution []Rule `yaml:"distribution"`
}

type Rule struct {
	Path      string `yaml:"path"`
	Immutable bool   `yaml:"immutable"`
}

type Builder interface {
	GetImmutables(phase string) []string
}

type DistroBuilder struct {
	Spec DistroRulesSpec
}

func NewDistroClusterRulesBuilder(distributionPath string) (*DistroBuilder, error) {
	rulesPath := filepath.Join(distributionPath, "rules", "kfddistribution-kfd-v1alpha2.yaml")

	spec, err := yamlx.FromFileV3[DistroRulesSpec](rulesPath)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", errReadingRulesFile, err)
	}

	return &DistroBuilder{
		Spec: spec,
	}, nil
}

func (r *DistroBuilder) GetImmutables(phase string) []string {
	switch phase {
	case "distribution":
		return r.extractImmutablesFromRules(r.Spec.Distribution)

	default:
		return []string{}
	}
}

func (*DistroBuilder) extractImmutablesFromRules(rules []Rule) []string {
	var immutables []string

	for _, rule := range rules {
		if rule.Immutable {
			immutables = append(immutables, rule.Path)
		}
	}

	return immutables
}
