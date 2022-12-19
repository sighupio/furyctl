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
	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
	netx "github.com/sighupio/furyctl/internal/x/net"
)

var (
	ErrDownloadingModule  = errors.New("error downloading module")
	ErrModuleHasNoVersion = errors.New("module has no version")
	ErrModuleHasNoName    = errors.New("module has no name")
)

func NewDownloader(client netx.Client, basePath, binPath string) *Downloader {
	return &Downloader{
		client:   client,
		basePath: basePath,
		binPath:  binPath,
		toolFactory: tools.NewFactory(execx.NewStdExecutor(), tools.FactoryPaths{
			Bin: filepath.Join(basePath, "vendor", "bin"),
		}),
	}
}

type Downloader struct {
	client      netx.Client
	toolFactory *tools.Factory
	basePath    string
	binPath     string
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
	newPrefix := "git@github.com:sighupio/kubernetes-fury"
	oldPrefix := "git@github.com:sighupio/fury-kubernetes"

	mods := reflect.ValueOf(modules)

	for i := 0; i < mods.NumField(); i++ {
		name := strings.ToLower(mods.Type().Field(i).Name)
		version, ok := mods.Field(i).Interface().(string)

		if !ok {
			return fmt.Errorf("%s: %w", name, ErrModuleHasNoVersion)
		}

		if name == "" {
			return ErrModuleHasNoName
		}

		errs := []error{}

		for _, prefix := range []string{oldPrefix, newPrefix} {
			src := fmt.Sprintf("git::%s-%s.git?ref=%s", prefix, name, version)

			if err := dd.client.Download(src, filepath.Join(dd.basePath, "vendor", "modules", name)); err != nil {
				errs = append(errs, fmt.Errorf("%w '%s': %v", distribution.ErrDownloadingFolder, src, err))

				continue
			}

			errs = []error{}

			break
		}

		if len(errs) > 0 {
			return fmt.Errorf("%w '%s': %v", ErrDownloadingModule, name, errs)
		}
	}

	return nil
}

func (dd *Downloader) DownloadInstallers(installers config.KFDKubernetes) error {
	insts := reflect.ValueOf(installers)

	for i := 0; i < insts.NumField(); i++ {
		name := strings.ToLower(insts.Type().Field(i).Name)

		v, ok := insts.Field(i).Interface().(config.KFDProvider)
		if !ok {
			return fmt.Errorf("%s: %w", name, ErrModuleHasNoVersion)
		}

		version := v.Installer

		src := fmt.Sprintf("git::git@github.com:sighupio/fury-%s-installer?ref=%s", name, version)

		if err := dd.client.Download(src, filepath.Join(dd.basePath, "vendor", "installers", name)); err != nil {
			return fmt.Errorf("%w '%s': %v", distribution.ErrDownloadingFolder, src, err)
		}
	}

	return nil
}

func (dd *Downloader) DownloadTools(kfdTools config.KFDTools) ([]string, error) {
	tls := reflect.ValueOf(kfdTools)

	unsupportedTools := []string{}

	for i := 0; i < tls.NumField(); i++ {
		for j := 0; j < tls.Field(i).NumField(); j++ {
			name := strings.ToLower(tls.Field(i).Type().Field(j).Name)

			version, ok := tls.Field(i).Field(j).Interface().(config.Tool)

			if !ok {
				return unsupportedTools, fmt.Errorf("%s: %w", name, ErrModuleHasNoVersion)
			}

			tool := dd.toolFactory.Create(name, version.String())
			if tool == nil || !tool.SupportsDownload() {
				unsupportedTools = append(unsupportedTools, name)

				continue
			}

			dst := filepath.Join(dd.binPath, name, version.String())

			if err := dd.client.Download(tool.SrcPath(), dst); err != nil {
				return unsupportedTools, fmt.Errorf("%w '%s': %v", distribution.ErrDownloadingFolder, tool.SrcPath(), err)
			}

			if err := tool.Rename(dst); err != nil {
				return unsupportedTools, fmt.Errorf("%w '%s': %v", distribution.ErrRenamingFile, tool.SrcPath(), err)
			}

			err := os.Chmod(filepath.Join(dst, name), iox.FullPermAccess)
			if err != nil {
				return unsupportedTools, fmt.Errorf("%w '%s': %v", distribution.ErrChangingFilePermissions, tool.SrcPath(), err)
			}
		}
	}

	return unsupportedTools, nil
}
