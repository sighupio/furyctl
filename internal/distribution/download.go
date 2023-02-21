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

const DefaultBaseURL = "git::git@github.com:sighupio/fury-distribution?ref=%s&depth=1"

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
	ErrUnsupportedVersion      = errors.New("unsupported KFD version")
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
	url := distroLocation

	// TODO: minimalConf cannot be validated using the furyctl config validation, we need a dedicated one.
	// if err := d.validate.Struct(minimalConf); err != nil {
	// 	return DownloadResult{}, fmt.Errorf("invalid furyctl config: %w", err)
	// }

	if distroLocation == "" {
		url = fmt.Sprintf(DefaultBaseURL, minimalConf.Spec.DistributionVersion)
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

	logrus.Debugf("Downloading '%s' in '%s'", src, dst)

	if err := netx.NewGoGetterClient().Download(src, dst); err != nil {
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
			return DownloadResult{}, fmt.Errorf("%w: %s is not supported by furyctl-ng, "+
				"try another version or use flag --distro-location to specify a custom location",
				ErrUnsupportedVersion,
				minimalConf.Spec.DistributionVersion,
			)
		}

		return DownloadResult{}, fmt.Errorf("%w: seems like %s is not supported by furyctl-ng, "+
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

	return DownloadResult{
		RepoPath:       dst,
		MinimalConf:    minimalConf,
		DistroManifest: kfdManifest,
	}, nil
}
