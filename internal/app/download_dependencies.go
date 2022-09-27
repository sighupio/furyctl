// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

import (
	"github.com/sighupio/furyctl/internal/dependencies"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/netx"
)

type DownloadDependenciesRequest struct {
	FuryctlBinVersion string
	DistroLocation    string
	FuryctlConfPath   string
	Debug             bool
}

type DownloadDependenciesResponse struct {
	DepsErrors []error
	UnsupTools []string
	RepoPath   string
}

func (v DownloadDependenciesResponse) HasErrors() bool {
	return len(v.DepsErrors) > 0
}

func NewDownloadDependencies(client netx.Client, basePath string) *DownloadDependencies {
	return &DownloadDependencies{
		client:   client,
		basePath: basePath,
	}
}

type DownloadDependencies struct {
	client   netx.Client
	basePath string
}

func (dd *DownloadDependencies) Execute(req DownloadDependenciesRequest) (DownloadDependenciesResponse, error) {
	distrodl := distribution.NewDownloader(dd.client, req.Debug)

	dres, err := distrodl.Download(req.FuryctlBinVersion, req.DistroLocation, req.FuryctlConfPath)
	if err != nil {
		return DownloadDependenciesResponse{}, err
	}

	depsdl := dependencies.NewDownloader(dd.client, dd.basePath)

	errs, ut := depsdl.DownloadAll(dres.DistroManifest)

	return DownloadDependenciesResponse{
		DepsErrors: errs,
		UnsupTools: ut,
		RepoPath:   dres.RepoPath,
	}, nil
}
