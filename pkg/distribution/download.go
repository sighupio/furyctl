// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package distribution

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/furyctl/configs"
	idist "github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/git"
	iox "github.com/sighupio/furyctl/internal/x/io"
	netx "github.com/sighupio/furyctl/pkg/x/net"
	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

const DefaultBaseURL = "git::%s/fury-distribution?ref=%s&depth=1"

var (
	ErrCannotDownloadDistribution = errors.New("cannot download distribution")
	ErrChangingFilePermissions    = errors.New("error changing file permissions")
	ErrCreatingTempDir            = errors.New("error creating temp dir")
	ErrDownloadingFolder          = errors.New("error downloading folder")
	ErrMergeCompleteConfig        = errors.New("error merging complete config")
	ErrMergeDistroConfig          = errors.New("error merging distribution config")
	ErrRenamingFile               = errors.New("error renaming file")
	ErrResolvingAbsPath           = errors.New("error resolving absolute path")
	ErrValidateConfig             = errors.New("error validating config")
	ErrWriteFile                  = errors.New("error writing file")
	ErrYamlMarshalFile            = errors.New("error marshaling yaml file")
	ErrYamlUnmarshalFile          = errors.New("error unmarshaling yaml file")
	ErrUnsupportedVersion         = errors.New("unsupported KFD version")
)

type DownloadResult struct {
	RepoPath       string
	MinimalConf    config.Furyctl
	DistroManifest config.KFD
}

func NewCachingDownloader(
	client netx.Client,
	outDir string,
	gitProtocol git.Protocol,
	customDistroPatchesPath string,
) *Downloader {
	return NewDownloader(
		netx.WithLocalCache(client, filepath.Join(outDir, ".furyctl", "cache")),
		gitProtocol,
		customDistroPatchesPath,
	)
}

func NewDownloader(client netx.Client, gitProtocol git.Protocol, customDistroPatchesPath string) *Downloader {
	return &Downloader{
		client:                  client,
		validate:                config.NewValidator(),
		gitProtocol:             gitProtocol,
		customDistroPatchesPath: customDistroPatchesPath,
	}
}

type Downloader struct {
	client                  netx.Client
	validate                *validator.Validate
	gitProtocol             git.Protocol
	customDistroPatchesPath string
}

func (d *Downloader) Download(
	distroLocation string,
	furyctlConfPath string,
) (DownloadResult, error) {
	minimalConf, err := yamlx.FromFileV3[config.Furyctl](furyctlConfPath)
	if err != nil {
		return DownloadResult{}, fmt.Errorf("%w: %w", ErrYamlUnmarshalFile, err)
	}

	compatChecker, err := idist.NewCompatibilityChecker(minimalConf.Spec.DistributionVersion, minimalConf.Kind)
	if err != nil {
		return DownloadResult{}, fmt.Errorf("%w: %w", ErrCannotDownloadDistribution, err)
	}

	if !compatChecker.IsCompatible() {
		logrus.Warnf("The specified KFD version %s is not supported by furyctl, "+
			"please upgrade furyctl to the latest version or use a supported version",
			minimalConf.Spec.DistributionVersion)
	}

	result, err := d.DoDownload(distroLocation, minimalConf)
	if err != nil {
		if errClear := d.client.Clear(); errClear != nil {
			logrus.Error(errClear)

			return DownloadResult{}, fmt.Errorf("%w: %w", ErrCannotDownloadDistribution, err)
		}

		return result, err
	}

	return result, nil
}

func (d *Downloader) DoDownload(
	distroLocation string,
	minimalConf config.Furyctl,
) (DownloadResult, error) {
	url := distroLocation

	protocol, err := git.RepoPrefixByProtocol(d.gitProtocol)
	if err != nil {
		return DownloadResult{}, fmt.Errorf("%w: %w", ErrCannotDownloadDistribution, err)
	}

	if distroLocation == "" {
		url = fmt.Sprintf(DefaultBaseURL, protocol, minimalConf.Spec.DistributionVersion)
	}

	if strings.HasPrefix(url, ".") {
		var err error
		if url, err = filepath.Abs(url); err != nil {
			return DownloadResult{}, fmt.Errorf("%w: %v", ErrResolvingAbsPath, err)
		}
	}

	baseDst, err := os.MkdirTemp("", "furyctl-")
	if err != nil {
		return DownloadResult{}, fmt.Errorf("%w: %v", ErrCreatingTempDir, err)
	}

	src := url
	dst := filepath.Join(baseDst, "data")

	if err := d.client.Download(src, dst); err != nil {
		if errors.Is(err, netx.ErrDownloadOptionsExhausted) {
			if distroLocation == "" {
				return DownloadResult{}, fmt.Errorf("%w: seems like the specified version "+
					"%s does not exist, try another version from the official repository",
					ErrUnsupportedVersion,
					minimalConf.Spec.DistributionVersion,
				)
			}

			return DownloadResult{}, fmt.Errorf("%w: seems like the specified location %s"+
				" does not exist, try another version from the official repository",
				ErrUnsupportedVersion,
				url,
			)
		}

		return DownloadResult{}, fmt.Errorf("%w '%s': %v", ErrDownloadingFolder, src, err)
	}

	kfdPath := filepath.Join(dst, "kfd.yaml")

	_, err = os.Stat(kfdPath)
	if os.IsNotExist(err) {
		if distroLocation == "" {
			return DownloadResult{}, fmt.Errorf("%w: %s is not supported by furyctl, "+
				"try another version or use flag --distro-location to specify a custom location",
				ErrUnsupportedVersion,
				minimalConf.Spec.DistributionVersion,
			)
		}

		return DownloadResult{}, fmt.Errorf("%w: seems like %s is not supported by furyctl, "+
			"try another version from the official repository",
			ErrUnsupportedVersion,
			distroLocation,
		)
	}

	kfdManifest, err := yamlx.FromFileV3[config.KFD](kfdPath)
	if err != nil {
		return DownloadResult{}, err
	}

	if err := d.validate.Struct(kfdManifest); err != nil {
		return DownloadResult{}, fmt.Errorf("invalid kfd config: %w", err)
	}

	if err := d.applyCompatibilityPatches(kfdManifest, dst); err != nil {
		return DownloadResult{}, fmt.Errorf("error applying compat patches: %w", err)
	}

	if d.customDistroPatchesPath != "" {
		if err := d.applyCustomCompatibilityPatches(kfdManifest, dst); err != nil {
			return DownloadResult{}, fmt.Errorf("error applying custom compat patches: %w", err)
		}
	}

	postPatchkfdManifest, err := yamlx.FromFileV3[config.KFD](kfdPath)
	if err != nil {
		return DownloadResult{}, err
	}

	return DownloadResult{
		RepoPath:       dst,
		MinimalConf:    minimalConf,
		DistroManifest: postPatchkfdManifest,
	}, nil
}

func (d *Downloader) applyCustomCompatibilityPatches(kfdManifest config.KFD, dst string) error {
	tmpDir, err := os.MkdirTemp("", "furyctl-custom-distro-patches-")
	if err != nil {
		return fmt.Errorf("error creating temp dir: %w", err)
	}

	if err := d.client.ClearItem(d.customDistroPatchesPath); err != nil {
		return fmt.Errorf("%w '%s': %w", ErrCannotDownloadDistribution, d.customDistroPatchesPath, err)
	}

	if err := d.client.Download(d.customDistroPatchesPath, tmpDir); err != nil {
		return fmt.Errorf("%w '%s': %w", ErrDownloadingFolder, d.customDistroPatchesPath, err)
	}

	patchesPath := path.Join(tmpDir, strings.ToLower(kfdManifest.Version))

	info, err := os.Stat(patchesPath)
	if err != nil {
		if os.IsNotExist(err) {
			logrus.Warnf("Cannot find a custom distribution patches directory for version %s in %s, skipping...",
				kfdManifest.Version,
				d.customDistroPatchesPath,
			)

			return nil
		}

		return fmt.Errorf("error getting custom distribution patches directory info: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("custom distribution patches location is not a directory: %w", err)
	}

	fsPatches, err := fs.Sub(os.DirFS(patchesPath), ".")
	if err != nil {
		return fmt.Errorf("error reading custom distribution patches directory: %w", err)
	}

	if err := iox.CopyRecursive(fsPatches, dst); err != nil {
		return fmt.Errorf("error copying custom distribution patches directory files: %w", err)
	}

	return nil
}

func (*Downloader) applyCompatibilityPatches(kfdManifest config.KFD, dst string) error {
	patchesPath := path.Join("patches", strings.ToLower(kfdManifest.Version))

	subFS, err := fs.Sub(configs.Tpl, patchesPath)
	if err != nil {
		return fmt.Errorf("error getting subfs: %w", err)
	}

	finfo, err := fs.Stat(subFS, ".")
	if err != nil {
		var perr *fs.PathError
		if errors.As(err, &perr) {
			return nil
		}

		return fmt.Errorf("error getting subfs stat: %w", err)
	}

	if finfo.IsDir() {
		if err := iox.CopyRecursive(subFS, dst); err != nil {
			return fmt.Errorf("error copying template files: %w", err)
		}
	}

	logrus.Infof("Compatibility patches applied for %s", kfdManifest.Version)

	return nil
}
