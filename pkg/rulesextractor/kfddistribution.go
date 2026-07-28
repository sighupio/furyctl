// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rulesextractor

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/sighupio/furyctl/internal/cluster"
	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

var ErrReadingRulesFile = errors.New("error while reading rules file")

type DistroExtractor struct {
	*BaseExtractor
}

func NewDistroClusterRulesExtractor(
	distributionPath string,
	renderedConfig map[string]any,
	supportedPhases cluster.SupportedPhases,
) (*DistroExtractor, error) {
	builder := &DistroExtractor{
		BaseExtractor: &BaseExtractor{
			RenderedConfig:  renderedConfig,
			SupportedPhases: supportedPhases,
		},
	}

	rulesPath := filepath.Join(distributionPath, "rules", "kfddistribution-kfd-v1alpha2.yaml")

	spec, err := yamlx.FromFileV3[Spec](rulesPath)
	if err != nil {
		return builder, fmt.Errorf("%w: %s", ErrReadingRulesFile, err)
	}

	builder.Spec = spec

	return builder, nil
}
