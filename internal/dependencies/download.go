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
	"time"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/furyctl/internal/dependencies/tools"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/git"
	"github.com/sighupio/furyctl/internal/semver"
	"github.com/sighupio/furyctl/internal/tool"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
	netx "github.com/sighupio/furyctl/internal/x/net"
)

const downloadsTimeout = 5 * time.Minute

var (
	ErrDownloadingModule  = errors.New("error downloading module")
	ErrDownloadTimeout    = fmt.Errorf("timeout while downloading")
	ErrModuleHasNoVersion = errors.New("module has no version")
	ErrModuleHasNoName    = errors.New("module has no name")
	ErrModuleNotFound     = errors.New("module not found")
)

func NewCachingDownloader(client netx.Client, outDir, basePath, binPath string, gitProtocol git.Protocol) *Downloader {
	return NewDownloader(netx.WithLocalCache(
		client,
		filepath.Join(outDir, ".furyctl", "cache")),
		basePath,
		binPath,
		gitProtocol,
	)
}

func NewDownloader(client netx.Client, basePath, binPath string, gitProtocol git.Protocol) *Downloader {
	return &Downloader{
		client:   client,
		basePath: basePath,
		binPath:  binPath,
		toolFactory: tools.NewFactory(execx.NewStdExecutor(), tools.FactoryPaths{
			Bin: filepath.Join(basePath, "vendor", "bin"),
		}),
		gitProtocol: gitProtocol,
	}
}

type Downloader struct {
	client      netx.Client
	toolFactory *tools.Factory
	basePath    string
	binPath     string
	gitProtocol git.Protocol
}

func (dd *Downloader) DownloadAll(kfd config.KFD) ([]error, []string) {
	errs := []error{}
	uts := []string{}

	vendorFolder := filepath.Join(dd.basePath, "vendor")

	logrus.Debug("Cleaning vendor folder")

	if err := iox.CheckDirIsEmpty(vendorFolder); err != nil {
		if err := os.RemoveAll(vendorFolder); err != nil {
			logrus.Debugf("Error while cleaning vendor folder: %v", err)

			return []error{fmt.Errorf("error removing folder: %w", err)}, nil
		}
	}

	gitPrefix, err := git.RepoPrefixByProtocol(dd.gitProtocol)
	if err != nil {
		return []error{err}, nil
	}

	utsCh := make(chan string)
	errCh := make(chan error)
	doneCh := make(chan bool)

	go func() {
		if err := dd.DownloadModules(kfd, gitPrefix); err != nil {
			errCh <- err
		}

		doneCh <- true
	}()

	go func() {
		if err := dd.DownloadInstallers(kfd.Kubernetes, gitPrefix); err != nil {
			errCh <- err
		}

		doneCh <- true
	}()

	go func() {
		uts, err := dd.DownloadTools(kfd)
		if err != nil {
			errCh <- err

			return
		}

		for _, ut := range uts {
			utsCh <- ut
		}

		doneCh <- true
	}()

	done := 0

	const todo = 3

	for {
		select {
		case err := <-errCh:
			errs = append(errs, err)

		case ut := <-utsCh:
			uts = append(uts, ut)

		case <-doneCh:
			done++

			if done == todo {
				if len(errs) > 0 {
					if errClear := dd.client.Clear(); errClear != nil {
						logrus.Error(errClear)
					}
				}

				return errs, uts
			}

		case <-time.After(downloadsTimeout):
			errs = append(errs, fmt.Errorf("%w dependencies", ErrDownloadTimeout))

			if errClear := dd.client.Clear(); errClear != nil {
				logrus.Error(errClear)
			}

			return errs, uts
		}
	}
}

func (dd *Downloader) DownloadModules(kfd config.KFD, gitPrefix string) error {
	oldPrefix := "kubernetes-fury"
	newPrefix := "fury-kubernetes"
	modules := kfd.Modules

	mods := reflect.ValueOf(modules)

	doneCh := make(chan bool)
	errCh := make(chan error)

	for i := 0; i < mods.NumField(); i++ {
		i := i

		go func(i int) {
			defer func() {
				doneCh <- true
			}()

			name := strings.ToLower(mods.Type().Field(i).Name)
			version, ok := mods.Field(i).Interface().(string)

			if !ok {
				errCh <- fmt.Errorf("%s: %w", name, ErrModuleHasNoVersion)

				return
			}

			if name == "" {
				errCh <- ErrModuleHasNoName

				return
			}

			if name == "tracing" && !distribution.HasFeature(kfd, distribution.FeatureTracingModule) {
				return
			}

			errs := []error{}
			retries := map[string]int{}

			dst := filepath.Join(dd.basePath, "vendor", "modules", name)

			for _, prefix := range []string{oldPrefix, newPrefix} {
				src := fmt.Sprintf("git::%s/%s-%s?ref=%s&depth=1", gitPrefix, prefix, name, version)

				moduleURL := createURL(prefix, name, version)

				req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, moduleURL, nil)
				if err != nil {
					errCh <- fmt.Errorf("%w '%s' (url: %s): %w", ErrDownloadingModule, name, moduleURL, err)

					return
				}

				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					if resp != nil && resp.Body != nil {
						if berr := resp.Body.Close(); berr != nil {
							errCh <- fmt.Errorf("%w '%s' (url: %s): %w, %w", ErrDownloadingModule, name, moduleURL, err, berr)

							return
						}
					}

					errCh <- fmt.Errorf("%w '%s' (url: %s): %w", ErrDownloadingModule, name, moduleURL, err)

					return
				}

				retries[name]++

				// Threshold to retry with the new prefix according to the fallback mechanism.
				threshold := 2

				if resp.StatusCode != http.StatusOK {
					if retries[name] >= threshold {
						errs = append(
							errs,
							fmt.Errorf(
								"%w '%s (url: %s)': please check if module exists or credentials are correctly configured",
								ErrModuleNotFound,
								name,
								moduleURL,
							),
						)
					}

					continue
				}

				if err := dd.client.Download(src, dst); err != nil {
					errs = append(errs, fmt.Errorf("%w '%s': %v", distribution.ErrDownloadingFolder, src, err))

					if _, err := os.Stat(dst); err == nil {
						if err := os.RemoveAll(dst); err != nil {
							logrus.Warningf("Error while cleaning up folder after failing download: %v", err)
						}
					}

					continue
				}

				errs = []error{}

				break
			}

			if len(errs) > 0 {
				errCh <- fmt.Errorf("%w '%s': %v", ErrDownloadingModule, name, errs)

				return
			}

			if err := os.RemoveAll(filepath.Join(dst, ".git")); err != nil {
				errCh <- fmt.Errorf("error removing .git subfolder: %w", err)

				return
			}
		}(i)
	}

	done := 0

	for {
		select {
		case <-doneCh:
			done++

			if done == mods.NumField() {
				return nil
			}

		case err := <-errCh:
			return err

		case <-time.After(downloadsTimeout):
			return fmt.Errorf("%w modules", ErrDownloadTimeout)
		}
	}
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

		// Rename the repository.
		if name == "onpremises" {
			src = fmt.Sprintf("git::%s/fury-kubernetes-on-premises?ref=%s&depth=1", gitPrefix, version)
		}

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

func (dd *Downloader) DownloadTools(kfd config.KFD) ([]string, error) {
	toolsCount := 0
	kfdTools := kfd.Tools
	tls := reflect.ValueOf(kfdTools)

	doneCh := make(chan bool)
	utsCh := make(chan string)
	errCh := make(chan error)

	for i := 0; i < tls.NumField(); i++ {
		i := i
		for j := 0; j < tls.Field(i).NumField(); j++ {
			j := j

			toolsCount++

			go func(i, j int) {
				defer func() {
					doneCh <- true
				}()

				name := strings.ToLower(tls.Field(i).Type().Field(j).Name)

				toolCfg, ok := tls.Field(i).Field(j).Interface().(config.KFDTool)

				if !ok {
					errCh <- fmt.Errorf("%s: %w", name, ErrModuleHasNoVersion)

					return
				}

				if (name == "helm" || name == "helmfile") && !distribution.HasFeature(kfd, distribution.FeaturePlugins) {
					return
				}

				if name == "yq" && !distribution.HasFeature(kfd, distribution.FeatureYqSupport) {
					return
				}

				if (name == "kapp") && !distribution.HasFeature(kfd, distribution.FeatureKappSupport) {
					return
				}

				tfc := dd.toolFactory.Create(tool.Name(name), toolCfg.Version)
				if tfc == nil || !tfc.SupportsDownload() {
					utsCh <- name

					return
				}

				dst := filepath.Join(dd.binPath, name, toolCfg.Version)

				if err := dd.client.Download(tfc.SrcPath(), dst); err != nil {
					errCh <- fmt.Errorf("%w '%s': %w", distribution.ErrDownloadingFolder, tfc.SrcPath(), err)

					return
				}

				if err := tfc.Rename(dst); err != nil {
					errCh <- fmt.Errorf("%w '%s': %w", distribution.ErrRenamingFile, tfc.SrcPath(), err)

					return
				}

				if _, err := os.Stat(filepath.Join(dst, name)); err == nil {
					if err := os.Chmod(filepath.Join(dst, name), iox.FullPermAccess); err != nil {
						errCh <- fmt.Errorf("%w '%s': %w", distribution.ErrChangingFilePermissions, filepath.Join(dst, name), err)

						return
					}
				}
			}(i, j)
		}
	}

	uts := []string{}
	done := 0

	for {
		select {
		case <-doneCh:
			done++

			if done == toolsCount {
				return uts, nil
			}

		case ut := <-utsCh:
			uts = append(uts, ut)

		case err := <-errCh:
			return uts, err

		case <-time.After(downloadsTimeout):
			return uts, fmt.Errorf("%w tools", ErrDownloadTimeout)
		}
	}
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
