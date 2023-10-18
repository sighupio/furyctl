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

type OnPremRulesSpec struct {
	Kubernetes   []Rule `yaml:"kubernetes"`
	Distribution []Rule `yaml:"distribution"`
}

type Rule struct {
	Path      string `yaml:"path"`
	Immutable bool   `yaml:"immutable"`
}

type Builder interface {
	GetImmutables(phase string) []string
}

type OnPremBuilder struct {
	Spec OnPremRulesSpec
}

func NewOnPremClusterRulesBuilder(distributionPath string) (*OnPremBuilder, error) {
	builder := OnPremBuilder{}

	rulesPath := filepath.Join(distributionPath, "rules", "onpremises-kfd-v1alpha2.yaml")

	spec, err := yamlx.FromFileV3[OnPremRulesSpec](rulesPath)
	if err != nil {
		return &builder, fmt.Errorf("%w: %s", ErrReadingRulesFile, err)
	}

	builder.Spec = spec

	return &builder, nil
}

func (r *OnPremBuilder) GetImmutables(phase string) []string {
	switch phase {
	case "kubernetes":
		return r.extractImmutablesFromRules(r.Spec.Kubernetes)

	case "distribution":
		return r.extractImmutablesFromRules(r.Spec.Distribution)

	default:
		return []string{}
	}
}

func (*OnPremBuilder) extractImmutablesFromRules(rules []Rule) []string {
	var immutables []string

	for _, rule := range rules {
		if rule.Immutable {
			immutables = append(immutables, rule.Path)
		}
	}

	return immutables
}
