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

	"github.com/sighupio/furyctl/internal/apis/config"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/git"
	"github.com/sighupio/furyctl/internal/semver"
	"github.com/sighupio/furyctl/internal/tool/mise"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
	dist "github.com/sighupio/furyctl/pkg/distribution"
	netx "github.com/sighupio/furyctl/pkg/x/net"
)

const downloadsTimeout = 5 * time.Minute

var (
	ErrDownloadingModule  = errors.New("error downloading module")
	ErrDownloadTimeout    = errors.New("timeout while downloading")
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
		client:      client,
		basePath:    basePath,
		binPath:     binPath,
		gitProtocol: gitProtocol,
	}
}

type Downloader struct {
	client      netx.Client
	basePath    string
	binPath     string
	gitProtocol git.Protocol
	// When true, drives mise with MISE_OFFLINE for air-gapped runs (false by default).
	offline bool
}

// SetOffline toggles offline mode: mise is driven with MISE_OFFLINE so it resolves tools from the
// already-populated data dir without any network access (air-gapped re-runs).
func (dd *Downloader) SetOffline(offline bool) {
	dd.offline = offline
}

func (dd *Downloader) DownloadAll(kfd config.KFD, kind string) ([]error, []string) {
	errs := []error{}
	uts := []string{}

	vendorFolder := filepath.Join(dd.basePath, "vendor")

	logrus.Debug("Cleaning vendor folder ", vendorFolder)

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
		if err := dd.DownloadModules(kfd, gitPrefix, kind); err != nil {
			errCh <- err
		}

		doneCh <- true
	}()

	go func() {
		if err := dd.DownloadInstallers(kfd.Kubernetes, gitPrefix, kind); err != nil {
			errCh <- err
		}

		doneCh <- true
	}()

	go func() {
		uts, err := dd.DownloadTools(kfd, kind)
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

func (dd *Downloader) DownloadModules(kfd config.KFD, gitPrefix, kind string) error {
	oldPrefix := "kubernetes-fury"
	newPrefix := "fury-kubernetes"
	modules := kfd.Modules

	mods := reflect.ValueOf(modules)

	doneCh := make(chan bool)
	errCh := make(chan error)

	for i := range mods.NumField() {
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

			if !distribution.ModuleNeededForKind(name, kind) {
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
					errs = append(errs, fmt.Errorf("%w '%s': %v", dist.ErrDownloadingFolder, src, err))

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

func (dd *Downloader) DownloadInstallers(installers config.KFDKubernetes, gitPrefix, kind string) error {
	insts := reflect.ValueOf(installers)

	for i := range insts.NumField() {
		name := strings.ToLower(insts.Type().Field(i).Name)

		// Only download the installer for the current cluster kind.
		if !distribution.InstallerNeededForKind(name, kind) {
			continue
		}

		dst := filepath.Join(dd.basePath, "vendor", "installers", name)

		v, ok := insts.Field(i).Interface().(config.KFDProvider)
		if !ok {
			return fmt.Errorf("%s: %w", name, ErrModuleHasNoVersion)
		}

		version := v.Installer
		src := fmt.Sprintf("git::%s/installer-%s?ref=%s&depth=1", gitPrefix, name, version)

		// Rename the repository.
		if name == "onpremises" {
			src = fmt.Sprintf("git::%s/fury-kubernetes-on-premises?ref=%s&depth=1", gitPrefix, version)
		}

		if err := dd.client.Download(src, dst); err != nil {
			return fmt.Errorf("%w '%s': %v", dist.ErrDownloadingFolder, src, err)
		}

		err := os.RemoveAll(filepath.Join(dst, ".git"))
		if err != nil {
			return fmt.Errorf("error removing .git subfolder: %w", err)
		}
	}

	return nil
}

// DownloadTools installs the tools needed by the cluster kind using the bundled mise, then
// materializes them into the legacy <binPath>/<tool>/<version>/<bin> layout (via relative symlinks)
// so the rest of furyctl (phase paths, runners, templates, validator) keeps working unchanged.
// Returns the host tools (uts) that mise does not manage and the operator must provide (e.g. awscli).
func (dd *Downloader) DownloadTools(kfd config.KFD, kind string) ([]string, error) {
	managed, uts := miseToolsForKind(kfd, kind)
	if len(managed) == 0 {
		return uts, nil
	}

	misePath, err := mise.EnsureBinary(dd.client, dd.binPath)
	if err != nil {
		return uts, fmt.Errorf("error ensuring mise binary: %w", err)
	}

	// The mise dir lives under binPath (next to the mise binary), NOT under vendor: vendor is wiped
	// on every DownloadAll, so keeping the installed tools here lets them cache across runs (and makes
	// --offline / air-gapped reuse work).
	miseDir := filepath.Join(dd.binPath, "mise")
	configFile := filepath.Join(miseDir, "mise.toml")

	if err := os.MkdirAll(miseDir, iox.FullPermAccess); err != nil {
		return uts, fmt.Errorf("error creating mise dir: %w", err)
	}

	if err := mise.WriteConfig(configFile, managed); err != nil {
		return uts, fmt.Errorf("error generating mise config: %w", err)
	}

	workDir, err := os.MkdirTemp("", "furyctl-mise-")
	if err != nil {
		return uts, fmt.Errorf("error creating mise workdir: %w", err)
	}

	defer os.RemoveAll(workDir)

	runner := mise.NewRunner(execx.NewStdExecutor(), mise.Paths{
		Mise:       misePath,
		DataDir:    filepath.Join(miseDir, "data"),
		CacheDir:   filepath.Join(miseDir, "cache"),
		ConfigFile: configFile,
		WorkDir:    workDir,
	}, dd.offline)

	// Stream mise's install progress into an ephemeral terminal region (TTY only); on debug or
	// --no-tty the region stays disabled and mise output is captured to the log file as usual.
	progress := iox.NewLiveRegion(os.Stderr, execx.NoTTY || logrus.GetLevel() >= logrus.DebugLevel)

	if err := runner.Install(progress); err != nil {
		progress.Clear()

		return uts, fmt.Errorf("error installing tools via mise: %w", err)
	}

	progress.Clear()

	logrus.Infof("Tools ready (%d installed via mise)", len(managed))

	for name, version := range managed {
		t := mise.ManagedTools[name]

		realPath, err := runner.Which(t.Bin)
		if err != nil {
			return uts, fmt.Errorf("error resolving tool '%s' via mise: %w", name, err)
		}

		if err := materializeTool(dd.binPath, name, version, t.Bin, realPath); err != nil {
			return uts, err
		}
	}

	return uts, nil
}

// miseToolsForKind partitions the tools needed by the kind into mise-managed (name -> version) and
// host tools (uts). Mirrors the per-kind/feature skip logic; when a tool is pinned in both common
// and eks, the eks (provider) value wins (union model / provider-overrides-common).
func miseToolsForKind(kfd config.KFD, kind string) (map[string]string, []string) {
	managed := map[string]string{}
	uts := []string{}

	tls := reflect.ValueOf(kfd.Tools)

	for i := range tls.NumField() {
		section := strings.ToLower(tls.Type().Field(i).Name)
		if !distribution.ToolSectionNeededForKind(section, kind) {
			continue
		}

		for j := range tls.Field(i).NumField() {
			name := strings.ToLower(tls.Field(i).Type().Field(j).Name)

			toolCfg, ok := tls.Field(i).Field(j).Interface().(config.KFDTool)
			if !ok || toolCfg.Version == "" {
				continue
			}

			if (name == "helm" || name == "helmfile") && !distribution.HasFeature(kfd, distribution.FeaturePlugins) {
				continue
			}

			if name == "yq" && !distribution.HasFeature(kfd, distribution.FeatureYqSupport) {
				continue
			}

			if name == "kapp" && !distribution.HasFeature(kfd, distribution.FeatureKappSupport) {
				continue
			}

			if mise.IsManaged(name) {
				managed[name] = toolCfg.Version
			} else {
				uts = append(uts, name)
			}
		}
	}

	return managed, uts
}

// materializeTool creates <binPath>/<name>/<version>/<bin> as a relative symlink to the mise-installed
// binary. Relative so the whole vendor dir can be moved (air-gapped) without breaking the link.
func materializeTool(binPath, name, version, bin, realPath string) error {
	dir := filepath.Join(binPath, name, version)
	if err := os.MkdirAll(dir, iox.FullPermAccess); err != nil {
		return fmt.Errorf("error creating tool dir %s: %w", dir, err)
	}

	link := filepath.Join(dir, bin)

	_ = os.Remove(link)

	target := realPath
	if rel, err := filepath.Rel(dir, realPath); err == nil {
		target = rel
	}

	if err := os.Symlink(target, link); err != nil {
		return fmt.Errorf("error linking tool %s: %w", link, err)
	}

	return nil
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
