// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/merge"
	"github.com/sighupio/furyctl/internal/netx"
	"github.com/sighupio/furyctl/internal/osx"
	"github.com/sighupio/furyctl/internal/schema/santhosh"
	"github.com/sighupio/furyctl/internal/yaml"
)

type ValidateConfigRequest struct {
	FuryctlBinVersion string
	DistroLocation    string
	FuryctlConfPath   string
	Debug             bool
}

type ValidateConfigResponse struct {
	Error    error
	RepoPath string
}

func (v ValidateConfigResponse) HasErrors() bool {
	return v.Error != nil
}

func NewValidateConfig(client netx.Client) *ValidateConfig {
	return &ValidateConfig{
		client: client,
	}
}

type ValidateConfig struct {
	client netx.Client
}

func (vc *ValidateConfig) Execute(req ValidateConfigRequest) (ValidateConfigResponse, error) {
	dloader := distribution.NewDownloader(vc.client, req.Debug)

	res, err := dloader.Download(req.FuryctlBinVersion, req.DistroLocation, req.FuryctlConfPath)
	if err != nil {
		return ValidateConfigResponse{}, err
	}

	schemaPath, err := distribution.GetSchemaPath(res.RepoPath, res.MinimalConf)
	if err != nil {
		return ValidateConfigResponse{}, err
	}

	defaultPath := distribution.GetDefaultPath(res.RepoPath)

	defaultedFuryctlConfPath, err := vc.mergeConfigAndDefaults(req.FuryctlConfPath, defaultPath)
	if err != nil {
		return ValidateConfigResponse{}, err
	}
	if !req.Debug {
		defer osx.CleanupTempDir(filepath.Base(defaultedFuryctlConfPath))
	}

	schema, err := santhosh.LoadSchema(schemaPath)
	if err != nil {
		return ValidateConfigResponse{}, err
	}

	conf, err := yaml.FromFileV3[any](defaultedFuryctlConfPath)
	if err != nil {
		return ValidateConfigResponse{}, err
	}

	if err := schema.ValidateInterface(conf); err != nil {
		return ValidateConfigResponse{
			RepoPath: res.RepoPath,
			Error:    err,
		}, nil
	}

	return ValidateConfigResponse{}, nil
}

func (vc *ValidateConfig) mergeConfigAndDefaults(furyctlFilePath, defaultsFilePath string) (string, error) {
	defaultsFile, err := yaml.FromFileV2[map[any]any](defaultsFilePath)
	if err != nil {
		return "", fmt.Errorf("%w: %v", distribution.ErrYamlUnmarshalFile, err)
	}

	furyctlFile, err := yaml.FromFileV2[map[any]any](furyctlFilePath)
	if err != nil {
		return "", fmt.Errorf("%w: %v", distribution.ErrYamlUnmarshalFile, err)
	}

	defaultsModel := merge.NewDefaultModel(defaultsFile, ".data")
	distributionModel := merge.NewDefaultModel(furyctlFile, ".spec.distribution")

	distroMerger := merge.NewMerger(defaultsModel, distributionModel)

	defaultedDistribution, err := distroMerger.Merge()
	if err != nil {
		return "", fmt.Errorf("%w: %v", distribution.ErrMergeDistroConfig, err)
	}

	furyctlModel := merge.NewDefaultModel(furyctlFile, ".spec.distribution")
	defaultedDistributionModel := merge.NewDefaultModel(defaultedDistribution, ".data")

	furyctlMerger := merge.NewMerger(furyctlModel, defaultedDistributionModel)

	defaultedFuryctl, err := furyctlMerger.Merge()
	if err != nil {
		return "", fmt.Errorf("%w: %v", distribution.ErrMergeCompleteConfig, err)
	}

	outYaml, err := yaml.MarshalV2(defaultedFuryctl)
	if err != nil {
		return "", fmt.Errorf("%w: %v", distribution.ErrYamlMarshalFile, err)
	}

	outDirPath, err := os.MkdirTemp("", "furyctl-defaulted-")
	if err != nil {
		return "", fmt.Errorf("%w: %v", distribution.ErrCreatingTempDir, err)
	}

	confPath := filepath.Join(outDirPath, "config.yaml")
	if err := os.WriteFile(confPath, outYaml, os.ModePerm); err != nil {
		return "", fmt.Errorf("%w: %v", distribution.ErrWriteFile, err)
	}

	return confPath, nil
}
