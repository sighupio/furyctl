// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

import (
	"errors"

	"github.com/santhosh-tekuri/jsonschema"

	"github.com/sighupio/furyctl/internal/config"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/netx"
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

	if err := config.Validate(req.FuryctlConfPath, res.RepoPath); err != nil {
		var terr *jsonschema.ValidationError

		if errors.As(err, &terr) {
			return ValidateConfigResponse{
				Error:    terr,
				RepoPath: res.RepoPath,
			}, nil
		}

		return ValidateConfigResponse{
			RepoPath: res.RepoPath,
		}, err
	}

	return ValidateConfigResponse{
		RepoPath: res.RepoPath,
	}, nil
}
