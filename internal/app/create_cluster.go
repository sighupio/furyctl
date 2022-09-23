// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

import (
	"errors"
	"path/filepath"

	"github.com/sighupio/furyctl/internal/app/validate"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/eks"
	"github.com/sighupio/furyctl/internal/execx"
)

var ErrUnsupportedDistributionKind = errors.New("unsupported distribution kind")

type CreateClusterRequest struct {
	DistroLocation    string
	FuryctlConfPath   string
	FuryctlBinVersion string
	Phase             string
	VpnAutoConnect    bool
	Debug             bool
}

type CreateClusterResponse struct {
	Error error
}

type CreateCluster struct{}

func NewCreateCluster() *CreateCluster {
	return &CreateCluster{}
}

func (v CreateClusterResponse) HasErrors() bool {
	return v.Error != nil
}

func (h *CreateCluster) Execute(req CreateClusterRequest) (CreateClusterResponse, error) {
	vendorPath, err := filepath.Abs("./vendor")
	if err != nil {
		return CreateClusterResponse{}, err
	}

	vc := NewValidateConfig()

	_, err = vc.Execute(ValidateConfigRequest{
		FuryctlBinVersion: req.FuryctlBinVersion,
		DistroLocation:    req.DistroLocation,
		FuryctlConfPath:   req.FuryctlConfPath,
		Debug:             req.Debug,
	})
	if err != nil {
		return CreateClusterResponse{}, err
	}

	vd := NewValidateDependencies(execx.NewStdExecutor())

	_, err = vd.Execute(ValidateDependenciesRequest{
		BinPath:           "",
		FuryctlBinVersion: req.FuryctlBinVersion,
		DistroLocation:    req.DistroLocation,
		FuryctlConfPath:   req.FuryctlConfPath,
		Debug:             req.Debug,
	})
	if err != nil {
		return CreateClusterResponse{}, err
	}

	err = DownloadRequirements(req.FuryctlConfPath, req.DistroLocation, vendorPath)
	if err != nil {
		return CreateClusterResponse{}, err
	}

	res, err := validate.DownloadDistro(req.FuryctlBinVersion, req.DistroLocation, req.FuryctlConfPath, req.Debug)
	if err != nil {
		return CreateClusterResponse{}, err
	}

	if res.MinimalConf.Kind.Equals(distribution.EKSCluster) {
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

		err = eksCluster.Create()
		if err != nil {
			return CreateClusterResponse{}, err
		}

		return CreateClusterResponse{}, nil
	}

	return CreateClusterResponse{
		Error: ErrUnsupportedDistributionKind,
	}, nil
}

func DownloadRequirements(configPath, distroLocation, dlPath string) error {
	return nil
}
