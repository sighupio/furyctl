// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package distribution

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/config"
	netx "github.com/sighupio/furyctl/internal/x/net"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

const DefaultBaseURL = "https://git@github.com/sighupio/fury-distribution?ref=%s"

var (
	ErrChangingFilePermissions = errors.New("error changing file permissions")
	ErrCreatingTempDir         = errors.New("error creating temp dir")
	ErrDownloadingFolder       = errors.New("error downloading folder")
	ErrMergeCompleteConfig     = errors.New("error merging complete config")
	ErrMergeDistroConfig       = errors.New("error merging distribution config")
	ErrRenamingFile            = errors.New("error renaming file")
	ErrResolvingAbsPath        = errors.New("error resolving absolute path")
	ErrValidateConfig          = errors.New("error validating config")
	ErrWriteFile               = errors.New("error writing file")
	ErrYamlMarshalFile         = errors.New("error marshaling yaml file")
	ErrYamlUnmarshalFile       = errors.New("error unmarshaling yaml file")
)

type DownloadResult struct {
	RepoPath       string
	MinimalConf    config.Furyctl
	DistroManifest config.KFD
}

func NewDownloader(client netx.Client) *Downloader {
	return &Downloader{
		client:   client,
		validate: config.NewValidator(),
	}
}

type Downloader struct {
	client   netx.Client
	validate *validator.Validate
}

func (d *Downloader) Download(
	distroLocation string,
	furyctlConfPath string,
) (DownloadResult, error) {
	minimalConf, err := yamlx.FromFileV3[config.Furyctl](furyctlConfPath)
	if err != nil {
		return DownloadResult{}, fmt.Errorf("%w: %s", ErrYamlUnmarshalFile, err)
	}

	return d.DoDownload(distroLocation, minimalConf)
}

func (d *Downloader) DoDownload(
	distroLocation string,
	minimalConf config.Furyctl,
) (DownloadResult, error) {
	if err := d.validate.Struct(minimalConf); err != nil {
		return DownloadResult{}, fmt.Errorf("invalid furyctl config: %w", err)
	}

	if distroLocation == "" {
		distroLocation = fmt.Sprintf(DefaultBaseURL, minimalConf.Spec.DistributionVersion)
	}

	if strings.HasPrefix(distroLocation, ".") {
		var err error
		if distroLocation, err = filepath.Abs(distroLocation); err != nil {
			return DownloadResult{}, fmt.Errorf("%w: %v", ErrResolvingAbsPath, err)
		}
	}

	baseDst, err := os.MkdirTemp("", "furyctl-")
	if err != nil {
		return DownloadResult{}, fmt.Errorf("%w: %v", ErrCreatingTempDir, err)
	}

	src := distroLocation
	dst := filepath.Join(baseDst, "data")

	logrus.Debugf("Downloading '%s' in '%s'", src, dst)

	if err := netx.NewGoGetterClient().Download(src, dst); err != nil {
		return DownloadResult{}, fmt.Errorf("%w '%s': %v", ErrDownloadingFolder, src, err)
	}

	kfdPath := filepath.Join(dst, "kfd.yaml")

	kfdManifest, err := yamlx.FromFileV3[config.KFD](kfdPath)
	if err != nil {
		return DownloadResult{}, err
	}

	if err := d.validate.Struct(kfdManifest); err != nil {
		return DownloadResult{}, fmt.Errorf("invalid kfd config: %w", err)
	}

	return DownloadResult{
		RepoPath:       dst,
		MinimalConf:    minimalConf,
		DistroManifest: kfdManifest,
	}, nil
}
