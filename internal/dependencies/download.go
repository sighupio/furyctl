// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dependencies

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/sighupio/fury-distribution/pkg/config"
	"github.com/sighupio/furyctl/internal/dependencies/tools"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/execx"
	"github.com/sighupio/furyctl/internal/netx"
)

var (
	ErrDownloadingModule  = errors.New("error downloading module")
	ErrModuleHasNoVersion = errors.New("module has no version")
	ErrModuleHasNoName    = errors.New("module has no name")
)

func NewDownloader(client netx.Client, basePath string) *Downloader {
	return &Downloader{
		client:   client,
		basePath: basePath,
		toolFactory: tools.NewFactory(execx.NewStdExecutor(), tools.FactoryPaths{
			Bin: filepath.Join(basePath, "vendor", "bin"),
		}),
	}
}

type Downloader struct {
	client      netx.Client
	toolFactory *tools.Factory
	basePath    string
}

func (dd *Downloader) DownloadAll(kfd config.KFD) ([]error, []string) {
	errs := []error{}
	if err := dd.DownloadModules(kfd.Modules); err != nil {
		errs = append(errs, err)
	}
	if err := dd.DownloadInstallers(kfd.Kubernetes); err != nil {
		errs = append(errs, err)
	}
	ut, err := dd.DownloadTools(kfd.Tools)
	if err != nil {
		errs = append(errs, err)
	}

	return errs, ut
}

func (dd *Downloader) DownloadModules(modules config.KFDModules) error {
	newPrefix := "https://github.com/sighupio/kubernetes-fury"
	oldPrefix := "https://github.com/sighupio/fury-kubernetes"

	mods := reflect.ValueOf(modules)

	for i := 0; i < mods.NumField(); i++ {
		name := strings.ToLower(mods.Type().Field(i).Name)
		version := mods.Field(i).Interface().(string)

		if name == "" {
			return ErrModuleHasNoName
		}

		if version == "" {
			return fmt.Errorf("%s: %w", name, ErrModuleHasNoVersion)
		}

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

func (dd *Downloader) DownloadInstallers(installers config.KFDKubernetes) error {
	insts := reflect.ValueOf(installers)

	for i := 0; i < insts.NumField(); i++ {
		name := strings.ToLower(insts.Type().Field(i).Name)
		version := insts.Field(i).Interface().(config.KFDProvider).Installer

		src := fmt.Sprintf("git::https://github.com/sighupio/fury-%s-installer?ref=%s", name, version)

		if err := dd.client.Download(src, filepath.Join(dd.basePath, "vendor", "installers", name)); err != nil {
			return err
		}
	}

	return nil
}

func (dd *Downloader) DownloadTools(tools config.KFDTools) ([]string, error) {
	tls := reflect.ValueOf(tools)

	unsupportedTools := []string{}
	for i := 0; i < tls.NumField(); i++ {
		name := strings.ToLower(tls.Type().Field(i).Name)
		version := tls.Field(i).Interface().(string)

		tool := dd.toolFactory.Create(name, version)
		if tool == nil || !tool.SupportsDownload() {
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

		err := os.Chmod(filepath.Join(dst, name), 0o755)
		if err != nil {
			return unsupportedTools, err
		}
	}

	return unsupportedTools, nil
}
