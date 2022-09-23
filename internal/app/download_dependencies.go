// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

import (
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/netx"
	"github.com/sighupio/furyctl/internal/tools"
)

var (
	ErrDownloadingModule = errors.New("error downloading module")
	ErrUnsupportedTools  = errors.New("unsupported tools")
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
	client      netx.Client
	toolFactory tools.Factory
	basePath    string
}

func (dd *DownloadDependencies) Execute(req DownloadDependenciesRequest) (DownloadDependenciesResponse, error) {
	dloader := distribution.NewDownloader(dd.client, req.Debug)

	dres, err := dloader.Download(req.FuryctlBinVersion, req.DistroLocation, req.FuryctlConfPath)
	if err != nil {
		return DownloadDependenciesResponse{}, err
	}

	errs := []error{}
	if err := dd.DownloadModules(dres.DistroManifest.Modules); err != nil {
		errs = append(errs, err)
	}
	if err := dd.DownloadInstallers(dres.DistroManifest.Kubernetes); err != nil {
		errs = append(errs, err)
	}
	ut, err := dd.DownloadTools(dres.DistroManifest.Tools)
	if err != nil {
		errs = append(errs, err)
	}

	return DownloadDependenciesResponse{
		DepsErrors: errs,
		UnsupTools: ut,
		RepoPath:   dres.RepoPath,
	}, nil
}

func (dd *DownloadDependencies) DownloadModules(modules distribution.ManifestModules) error {
	newPrefix := "https://github.com/sighupio/kubernetes-fury"
	oldPrefix := "https://github.com/sighupio/fury-kubernetes"

	mods := reflect.ValueOf(modules)

	for i := 0; i < mods.NumField(); i++ {
		name := strings.ToLower(mods.Type().Field(i).Name)
		version := mods.Field(i).Interface().(string)

		errors := []error{}
		for _, prefix := range []string{oldPrefix, newPrefix} {
			src := fmt.Sprintf("git::%s-%s.git?ref=%s", prefix, name, version)

			if err := dd.client.Download(src, filepath.Join(dd.basePath, "vendor", "modules", name)); err != nil {
				errors = append(errors, fmt.Errorf("%w '%s': %v", distribution.ErrDownloadingFolder, src, err))
				continue
			}

			errors = []error{}
			break
		}

		if len(errors) > 0 {
			return fmt.Errorf("%w '%s': %v", ErrDownloadingModule, name, errors)
		}
	}

	return nil
}

func (dd *DownloadDependencies) DownloadInstallers(installers distribution.ManifestKubernetes) error {
	insts := reflect.ValueOf(installers)

	for i := 0; i < insts.NumField(); i++ {
		name := strings.ToLower(insts.Type().Field(i).Name)
		version := insts.Field(i).Interface().(distribution.ManifestProvider).Installer

		src := fmt.Sprintf("git::https://github.com/sighupio/fury-%s-installer?ref=%s", name, version)

		if err := dd.client.Download(src, filepath.Join(dd.basePath, "vendor", "installers", name)); err != nil {
			return err
		}
	}

	return nil
}

func (dd *DownloadDependencies) DownloadTools(tools distribution.ManifestTools) ([]string, error) {
	tls := reflect.ValueOf(tools)

	unsupportedTools := []string{}
	for i := 0; i < tls.NumField(); i++ {
		name := strings.ToLower(tls.Type().Field(i).Name)
		version := tls.Field(i).Interface().(string)

		tool := dd.toolFactory.Create(name, version)
		if tool == nil {
			unsupportedTools = append(unsupportedTools, name)
			continue
		}

		dst := filepath.Join(dd.basePath, "vendor", "bin")

		if err := dd.client.Download(tool.SrcPath(), dst); err != nil {
			return unsupportedTools, err
		}

		if err := tool.Rename(dst); err != nil {
			return unsupportedTools, err
		}
	}

	return unsupportedTools, nil
}
