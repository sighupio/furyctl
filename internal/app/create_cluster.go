// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

import (
	"errors"
	"path/filepath"

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
	vendorPath, err := filepath.Abs("./vendor")
	if err != nil {
		return CreateClusterResponse{}, err
	}

	dloader := distribution.NewDownloader(h.client, req.Debug)

	res, err := dloader.Download(req.FuryctlBinVersion, req.DistroLocation, req.FuryctlConfPath)
	if err != nil {
		return CreateClusterResponse{}, err
	}

	vc := NewValidateConfig(h.client)

	_, err = vc.Execute(ValidateConfigRequest{
		FuryctlBinVersion: req.FuryctlBinVersion,
		DistroLocation:    res.RepoPath,
		FuryctlConfPath:   req.FuryctlConfPath,
		Debug:             req.Debug,
	})
	if err != nil {
		return CreateClusterResponse{}, err
	}

	dl := NewDownloadDependencies(h.client, vendorPath)
	_, err = dl.Execute(DownloadDependenciesRequest{
		FuryctlBinVersion: req.FuryctlBinVersion,
		DistroLocation:    res.RepoPath,
		FuryctlConfPath:   req.FuryctlConfPath,
		Debug:             req.Debug,
	})
	if err != nil {
		return CreateClusterResponse{}, err
	}

	vd := NewValidateDependencies(h.client, h.executor)

	_, err = vd.Execute(ValidateDependenciesRequest{
		BinPath:           vendorPath,
		FuryctlBinVersion: req.FuryctlBinVersion,
		DistroLocation:    res.RepoPath,
		FuryctlConfPath:   req.FuryctlConfPath,
		Debug:             req.Debug,
	})
	if err != nil {
		return CreateClusterResponse{}, err
	}

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

		err = eksCluster.Create(req.DryRun)
		if err != nil {
			return CreateClusterResponse{}, err
		}

		return CreateClusterResponse{}, nil
	}

	return CreateClusterResponse{
		Error: ErrUnsupportedDistributionKind,
	}, nil
}
