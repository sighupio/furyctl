// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

import (
	"errors"

	"github.com/sighupio/furyctl/internal/dependencies/envvars"
	"github.com/sighupio/furyctl/internal/dependencies/tools"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/execx"
	"github.com/sighupio/furyctl/internal/netx"
)

var (
	ErrEmptyToolVersion = errors.New("empty tool version")
	ErrMissingEnvVar    = errors.New("missing environment variable")
	ErrWrongToolVersion = errors.New("wrong tool version")
)

type ValidateDependenciesRequest struct {
	BinPath           string
	FuryctlBinVersion string
	DistroLocation    string
	FuryctlConfPath   string
	Debug             bool
}

type ValidateDependenciesResponse struct {
	Errors   []error
	RepoPath string
}

func (vdr *ValidateDependenciesResponse) appendErrors(errs []error) {
	vdr.Errors = append(vdr.Errors, errs...)
}

func (vdr *ValidateDependenciesResponse) HasErrors() bool {
	return len(vdr.Errors) > 0
}

func NewValidateDependencies(client netx.Client, executor execx.Executor) *ValidateDependencies {
	return &ValidateDependencies{
		client:   client,
		executor: executor,
	}
}

type ValidateDependencies struct {
	client   netx.Client
	executor execx.Executor
}

func (vd *ValidateDependencies) Execute(req ValidateDependenciesRequest) (ValidateDependenciesResponse, error) {
	dloader := distribution.NewDownloader(vd.client, req.Debug)

	res := ValidateDependenciesResponse{}

	dres, err := dloader.Download(req.FuryctlBinVersion, req.DistroLocation, req.FuryctlConfPath)
	if err != nil {
		return res, err
	}

	toolsValidator := tools.NewValidator(vd.executor)
	envVarsValidator := envvars.NewValidator()

	res.RepoPath = dres.RepoPath
	res.appendErrors(toolsValidator.Validate(dres.DistroManifest, req.BinPath))
	res.appendErrors(envVarsValidator.Validate(dres.MinimalConf.Kind))

	return res, nil
}
