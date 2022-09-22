// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"

	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/netx"
	"github.com/sighupio/furyctl/internal/semver"
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
	Error    error
	RepoPath string
}

func (v DownloadDependenciesResponse) HasErrors() bool {
	return v.Error != nil
}

func NewDownloadDependencies(client netx.Client) *DownloadDependencies {
	return &DownloadDependencies{
		client: client,
	}
}

type DownloadDependencies struct {
	client netx.Client
}

func (dd *DownloadDependencies) Execute(req DownloadDependenciesRequest) (DownloadDependenciesResponse, error) {
	dres, err := distribution.Download(req.FuryctlBinVersion, req.DistroLocation, req.FuryctlConfPath, req.Debug)
	if err != nil {
		return DownloadDependenciesResponse{}, err
	}

	if err := dd.DownloadModules(dres.DistroManifest.Modules); err != nil {
		return DownloadDependenciesResponse{}, err
	}
	if err := dd.DownloadInstallers(dres.DistroManifest.Kubernetes); err != nil {
		return DownloadDependenciesResponse{}, err
	}
	if err := dd.DownloadTools(dres.DistroManifest.Tools); err != nil {
		return DownloadDependenciesResponse{}, err
	}

	return DownloadDependenciesResponse{
		Error:    nil,
		RepoPath: dres.RepoPath,
	}, nil
}

func (dd *DownloadDependencies) DownloadModules(modules distribution.ManifestModules) error {
	defaultPrefix := "https://github.com/sighupio/kubernetes-fury"
	fallbackPrefix := "https://github.com/sighupio/fury-kubernetes"

	basePath, err := os.Getwd()
	if err != nil {
		return err
	}

	mods := reflect.ValueOf(modules)

	for i := 0; i < mods.NumField(); i++ {
		name := strings.ToLower(mods.Type().Field(i).Name)
		version := mods.Field(i).Interface().(string)

		errors := []error{}
		for _, prefix := range []string{defaultPrefix, fallbackPrefix} {
			src := fmt.Sprintf("git::%s-%s.git?ref=%s", prefix, name, version)

			if err := dd.client.Download(src, filepath.Join(basePath, "vendor", "modules", name)); err != nil {
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
	basePath, err := os.Getwd()
	if err != nil {
		return err
	}

	insts := reflect.ValueOf(installers)

	for i := 0; i < insts.NumField(); i++ {
		name := strings.ToLower(insts.Type().Field(i).Name)
		version := insts.Field(i).Interface().(distribution.ManifestProvider).Installer

		src := fmt.Sprintf("git::https://github.com/sighupio/fury-%s-installer?ref=%s", name, version)

		if err := dd.client.Download(src, filepath.Join(basePath, "vendor", "installers", name)); err != nil {
			return err
		}
	}

	return nil
}

func (dd *DownloadDependencies) DownloadTools(tools distribution.ManifestTools) error {
	basePath, err := os.Getwd()
	if err != nil {
		return err
	}

	tls := reflect.ValueOf(tools)

	unsupportedTools := []string{}
	for i := 0; i < tls.NumField(); i++ {
		name := strings.ToLower(tls.Type().Field(i).Name)
		version := tls.Field(i).Interface().(string)

		src, dstName := dd.fmtPaths(name, version, runtime.GOOS, "amd64")
		if src == "" {
			unsupportedTools = append(unsupportedTools, name)
			continue
		}

		dst := filepath.Join(basePath, "vendor", "bin", name)

		if err := dd.client.Download(src, dst); err != nil {
			return err
		}

		if dstName != name {
			if err := os.Rename(filepath.Join(dst, dstName), filepath.Join(dst, name)); err != nil {
				return err
			}
		}
	}

	if unsupportedTools != nil {
		return fmt.Errorf("%w: %v", ErrUnsupportedTools, unsupportedTools)
	}

	return nil
}

// fmtPaths returns the final location of the tool to download and the name of the downloaded binary
func (dd *DownloadDependencies) fmtPaths(toolName, toolVersion, osName, archName string) (string, string) {
	if toolName == "furyagent" {
		return fmt.Sprintf(
			"https://github.com/sighupio/furyagent/releases/download/%s/furyagent-%s-%s",
			semver.EnsurePrefix(toolVersion, "v"),
			osName,
			archName,
		), fmt.Sprintf("furyagent-%s-%s", osName, archName)
	}

	if toolName == "kubectl" {
		return fmt.Sprintf(
			"https://dl.k8s.io/release/%s/bin/%s/%s/kubectl",
			semver.EnsurePrefix(toolVersion, "v"),
			osName,
			archName,
		), "kubectl"
	}

	if toolName == "kustomize" {
		return fmt.Sprintf(
			"https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize/%s/kustomize_%s_%s_%s.tar.gz",
			semver.EnsurePrefix(toolVersion, "v"),
			semver.EnsurePrefix(toolVersion, "v"),
			osName,
			archName,
		), "kustomize"
	}

	if toolName == "terraform" {
		return fmt.Sprintf(
			"https://releases.hashicorp.com/terraform/%s/terraform_%s_%s_%s.zip",
			semver.EnsureNoPrefix(toolVersion, "v"),
			semver.EnsureNoPrefix(toolVersion, "v"),
			osName,
			archName,
		), "terraform"
	}

	return "", ""
}
