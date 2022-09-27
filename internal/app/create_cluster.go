// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sighupio/furyctl/internal/config"
	"github.com/sighupio/furyctl/internal/dependencies"
	"github.com/sighupio/furyctl/internal/dependencies/envvars"
	"github.com/sighupio/furyctl/internal/dependencies/tools"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/eks"
	"github.com/sighupio/furyctl/internal/execx"
	"github.com/sighupio/furyctl/internal/netx"
)

var ErrUnsupportedDistributionKind = errors.New("unsupported distribution kind")

type CreateClusterRequest struct {
	DistroLocation    string
	FuryctlConfPath   string
	FuryctlBinVersion string
	Phase             string
	DryRun            bool
	VpnAutoConnect    bool
	Debug             bool
}

type CreateClusterResponse struct {
	Error error
}

type CreateCluster struct {
	client   netx.Client
	executor execx.Executor
}

func NewCreateCluster(client netx.Client, executor execx.Executor) *CreateCluster {
	return &CreateCluster{
		client:   client,
		executor: executor,
	}
}

func (v CreateClusterResponse) HasErrors() bool {
	return v.Error != nil
}

func (h *CreateCluster) Execute(req CreateClusterRequest) (CreateClusterResponse, error) {
	basePath, err := os.Getwd()
	if err != nil {
		return CreateClusterResponse{}, err
	}

	vendorPath := filepath.Join(basePath, "vendor")

	// Init downloaders
	distrodl := distribution.NewDownloader(h.client, req.Debug)
	depsdl := dependencies.NewDownloader(h.client, basePath)

	// Download the distribution
	res, err := distrodl.Download(req.FuryctlBinVersion, req.DistroLocation, req.FuryctlConfPath)
	if err != nil {
		return CreateClusterResponse{}, err
	}

	// Validate the furyctl.yaml file
	if err := config.Validate(req.FuryctlConfPath, res.RepoPath); err != nil {
		return CreateClusterResponse{}, err
	}

	// Download the dependencies
	if errs, _ := depsdl.DownloadAll(res.DistroManifest); len(errs) > 0 {
		return CreateClusterResponse{}, fmt.Errorf("errors downloading dependencies: %v", errs)
	}

	// Validate the dependencies
	if err := h.validateDependencies(res, vendorPath); err != nil {
		return CreateClusterResponse{}, err
	}

	// Create the cluster
	if res.MinimalConf.Kind == "EKSCluster" {
		eksCluster, err := eks.NewClusterCreator(
			res.MinimalConf.ApiVersion,
			req.Phase,
			res.DistroManifest,
			req.FuryctlConfPath,
			req.VpnAutoConnect,
		)
		if err != nil {
			return CreateClusterResponse{}, err
		}

		if err := eksCluster.Create(req.DryRun); err != nil {
			return CreateClusterResponse{}, err
		}

		return CreateClusterResponse{}, nil
	}

	return CreateClusterResponse{
		Error: ErrUnsupportedDistributionKind,
	}, nil
}

func (h *CreateCluster) validateDependencies(res distribution.DownloadResult, vendorPath string) error {
	toolsValidator := tools.NewValidator(h.executor)
	envVarsValidator := envvars.NewValidator()

	binPath := filepath.Join(vendorPath, "bin")

	if errs := toolsValidator.Validate(res.DistroManifest, binPath); len(errs) > 0 {
		return fmt.Errorf("errors validating tools: %v", errs)
	}

	if errs := envVarsValidator.Validate(res.MinimalConf.Kind); len(errs) > 0 {
		return fmt.Errorf("errors validating env vars: %v", errs)
	}

	return nil
}
