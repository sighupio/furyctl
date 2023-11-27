// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package v1alpha2

import (
	"fmt"
	"path"
	"strings"

	"github.com/sighupio/furyctl/internal/merge"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

type Merger interface {
	Create() (*merge.Merger, error)
}

type BaseMerger struct {
	DistributionPath string
	Kind             string
	ConfigPath       string
}

func NewBaseMerger(distributionPath, kind, configPath string) *BaseMerger {
	return &BaseMerger{
		DistributionPath: distributionPath,
		Kind:             kind,
		ConfigPath:       configPath,
	}
}

func (m *BaseMerger) Create() (*merge.Merger, error) {
	defaultsFilePath := path.Join(
		m.DistributionPath,
		"defaults",
		fmt.Sprintf("%s-kfd-v1alpha2.yaml", strings.ToLower(m.Kind)),
	)

	defaultsFile, err := yamlx.FromFileV2[map[any]any](defaultsFilePath)
	if err != nil {
		return &merge.Merger{}, fmt.Errorf("%s - %w", defaultsFilePath, err)
	}

	furyctlConf, err := yamlx.FromFileV2[map[any]any](m.ConfigPath)
	if err != nil {
		return &merge.Merger{}, fmt.Errorf("%s - %w", m.ConfigPath, err)
	}

	merger := merge.NewMerger(
		merge.NewDefaultModel(defaultsFile, ".data"),
		merge.NewDefaultModel(furyctlConf, ".spec.distribution"),
	)

	_, err = merger.Merge()
	if err != nil {
		return nil, fmt.Errorf("error merging furyctl config: %w", err)
	}

	reverseMerger := merge.NewMerger(
		*merger.GetCustom(),
		*merger.GetBase(),
	)

	_, err = reverseMerger.Merge()
	if err != nil {
		return nil, fmt.Errorf("error merging furyctl config: %w", err)
	}

	return reverseMerger, nil
}
