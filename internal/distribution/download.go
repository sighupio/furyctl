// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package distribution

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-playground/validator/v10"
	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/config"
	"github.com/sighupio/furyctl/internal/netx"
	"github.com/sighupio/furyctl/internal/osx"
	"github.com/sighupio/furyctl/internal/semver"
	"github.com/sighupio/furyctl/internal/yaml"
)

const DefaultBaseUrl = "https://git@github.com/sighupio/fury-distribution?ref=%s"

var (
	ErrCreatingTempDir     = errors.New("error creating temp dir")
	ErrDownloadingFolder   = errors.New("error downloading folder")
	ErrMergeCompleteConfig = errors.New("error merging complete config")
	ErrMergeDistroConfig   = errors.New("error merging distribution config")
	ErrWriteFile           = errors.New("error writing file")
	ErrYamlMarshalFile     = errors.New("error marshaling yaml file")
	ErrYamlUnmarshalFile   = errors.New("error unmarshaling yaml file")
)

type DownloadResult struct {
	RepoPath       string
	MinimalConf    config.Furyctl
	DistroManifest config.KFD
}

func NewDownloader(client netx.Client, debug bool) *Downloader {
	return &Downloader{
		client:   client,
		validate: config.NewValidator(),
		debug:    debug,
	}
}

type Downloader struct {
	client   netx.Client
	validate *validator.Validate
	debug    bool
}

func (d *Downloader) Download(
	furyctlBinVersion string,
	distroLocation string,
	furyctlConfPath string,
) (DownloadResult, error) {
	minimalConf, err := yaml.FromFileV3[config.Furyctl](furyctlConfPath)
	if err != nil {
		return DownloadResult{}, err
	}

	if err := d.validate.Struct(minimalConf); err != nil {
		return DownloadResult{}, err
	}

	furyctlConfVersion := minimalConf.Spec.DistributionVersion

	if furyctlBinVersion != "unknown" {
		if !semver.SameMinorStr(furyctlConfVersion, furyctlBinVersion) {
			logrus.Warnf(
				"this version of furyctl ('%s') does not support distribution version '%s', results may be inaccurate",
				furyctlBinVersion,
				furyctlConfVersion,
			)
		}
	}

	if distroLocation == "" {
		distroLocation = fmt.Sprintf(DefaultBaseUrl, furyctlConfVersion)
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

	if !d.debug {
		defer osx.CleanupTempDir(filepath.Base(dst))
	}

	kfdPath := filepath.Join(dst, "kfd.yaml")
	kfdManifest, err := yaml.FromFileV3[config.KFD](kfdPath)
	if err != nil {
		return DownloadResult{}, err
	}

	if err := d.validate.Struct(kfdManifest); err != nil {
		return DownloadResult{}, err
	}

	if !semver.SamePatchStr(furyctlConfVersion, kfdManifest.Version) {
		return DownloadResult{}, fmt.Errorf(
			"versions mismatch: furyctl.yaml = '%s', furyctl binary = '%s'",
			furyctlConfVersion,
			kfdManifest.Version,
		)
	}

	return DownloadResult{
		RepoPath:       dst,
		MinimalConf:    minimalConf,
		DistroManifest: kfdManifest,
	}, nil
}
