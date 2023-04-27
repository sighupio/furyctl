// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dependencies

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/config"
	"github.com/sighupio/furyctl/internal/dependencies/tools"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/semver"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
	netx "github.com/sighupio/furyctl/internal/x/net"
)

const (
	GithubSSHRepoPrefix   = "git@github.com:sighupio"
	GithubHTTPSRepoPrefix = "https://github.com/sighupio"
)

var (
	ErrDownloadingModule  = errors.New("error downloading module")
	ErrModuleHasNoVersion = errors.New("module has no version")
	ErrModuleHasNoName    = errors.New("module has no name")
	ErrModuleNotFound     = errors.New("module not found")
)

func NewDownloader(client netx.Client, basePath, binPath string, https bool) *Downloader {
	return &Downloader{
		client:   client,
		basePath: basePath,
		binPath:  binPath,
		toolFactory: tools.NewFactory(execx.NewStdExecutor(), tools.FactoryPaths{
			Bin: filepath.Join(basePath, "vendor", "bin"),
		}),
		HTTPS: https,
	}
}

type Downloader struct {
	client      netx.Client
	toolFactory *tools.Factory
	basePath    string
	binPath     string
	HTTPS       bool
}

func (dd *Downloader) DownloadAll(kfd config.KFD) ([]error, []string) {
	errs := []error{}

	vendorFolder := filepath.Join(dd.basePath, "vendor")

	logrus.Debug("Cleaning vendor folder")

	if err := iox.CheckDirIsEmpty(vendorFolder); err != nil {
		err = os.RemoveAll(vendorFolder)
		if err != nil {
			logrus.Debugf("Error while cleaning vendor folder: %v", err)

			return []error{fmt.Errorf("error removing folder: %w", err)}, nil
		}
	}

	gitPrefix := GithubSSHRepoPrefix

	if dd.HTTPS {
		gitPrefix = GithubHTTPSRepoPrefix
	}

	if err := dd.DownloadModules(kfd.Modules, gitPrefix); err != nil {
		errs = append(errs, err)
	}

	if err := dd.DownloadInstallers(kfd.Kubernetes, gitPrefix); err != nil {
		errs = append(errs, err)
	}

	ut, err := dd.DownloadTools(kfd.Tools)
	if err != nil {
		errs = append(errs, err)
	}

	return errs, ut
}

func (dd *Downloader) DownloadModules(modules config.KFDModules, gitPrefix string) error {
	oldPrefix := "kubernetes-fury"
	newPrefix := "fury-kubernetes"

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
		retries := map[string]int{}

		dst := filepath.Join(dd.basePath, "vendor", "modules", name)

		for _, prefix := range []string{oldPrefix, newPrefix} {
			src := fmt.Sprintf("git::%s/%s-%s?ref=%s&depth=1", gitPrefix, prefix, name, version)

			req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, createURL(prefix, name, version), nil)
			if err != nil {
				return fmt.Errorf("%w '%s': %v", ErrDownloadingModule, name, err)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				if err := resp.Body.Close(); err != nil {
					return fmt.Errorf("%w '%s': %v", ErrDownloadingModule, name, err)
				}

				return fmt.Errorf("%w '%s': %v", ErrDownloadingModule, name, err)
			}

			retries[name]++

			// Threshold to retry with the new prefix according to the fallback mechanism.
			threshold := 2

			if resp.StatusCode != http.StatusOK {
				if retries[name] >= threshold {
					errs = append(errs, fmt.Errorf("%w '%s': please check if module exists or credentials are correctly configured",
						ErrModuleNotFound, name))
				}

				continue
			}

			if err := dd.client.Download(src, dst); err != nil {
				errs = append(errs, fmt.Errorf("%w '%s': %v", distribution.ErrDownloadingFolder, src, err))

				continue
			}

			errs = []error{}

			break
		}

		if len(errs) > 0 {
			return fmt.Errorf("%w '%s': %v", ErrDownloadingModule, name, errs)
		}

		err := os.RemoveAll(filepath.Join(dst, ".git"))
		if err != nil {
			return fmt.Errorf("error removing .git subfolder: %w", err)
		}
	}

	return nil
}

func (dd *Downloader) DownloadInstallers(installers config.KFDKubernetes, gitPrefix string) error {
	insts := reflect.ValueOf(installers)

	for i := 0; i < insts.NumField(); i++ {
		name := strings.ToLower(insts.Type().Field(i).Name)

		dst := filepath.Join(dd.basePath, "vendor", "installers", name)

		v, ok := insts.Field(i).Interface().(config.KFDProvider)
		if !ok {
			return fmt.Errorf("%s: %w", name, ErrModuleHasNoVersion)
		}

		version := v.Installer

		src := fmt.Sprintf("git::%s/fury-%s-installer?ref=%s&depth=1", gitPrefix, name, version)

		if err := dd.client.Download(src, dst); err != nil {
			return fmt.Errorf("%w '%s': %v", distribution.ErrDownloadingFolder, src, err)
		}

		err := os.RemoveAll(filepath.Join(dst, ".git"))
		if err != nil {
			return fmt.Errorf("error removing .git subfolder: %w", err)
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

func createURL(prefix, name, version string) string {
	ver := semver.EnsurePrefix(version)

	kindPrefix := "releases/tag"

	_, err := semver.NewVersion(ver)
	if err != nil {
		kindPrefix = "tree"
	}

	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return fmt.Sprintf("https://oauth2:%s@github.com/sighupio/%s-%s/%s/%s", token, prefix, name, kindPrefix, version)
	}

	return fmt.Sprintf("https://github.com/sighupio/%s-%s/%s/%s", prefix, name, kindPrefix, version)
}
