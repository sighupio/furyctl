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

	"github.com/sirupsen/logrus"

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

type downloadResult struct {
	RepoPath       string
	MinimalConf    FuryctlConfig
	DistroManifest Manifest
}

func Download(
	furyctlBinVersion string,
	distroLocation string,
	furyctlConfPath string,
	debug bool,
) (downloadResult, error) {
	minimalConf, err := yaml.FromFileV3[FuryctlConfig](furyctlConfPath)
	if err != nil {
		return downloadResult{}, err
	}

	furyctlConfSemVer := minimalConf.Spec.DistributionVersion

	if furyctlBinVersion != "unknown" {
		furyctlBinSemVer, err := semver.NewVersion(fmt.Sprintf("v%s", furyctlBinVersion))
		if err != nil {
			return downloadResult{}, err
		}

		if !semver.SameMinor(furyctlConfSemVer, furyctlBinSemVer) {
			logrus.Warnf(
				"this version of furyctl ('%s') does not support distribution version '%s', results may be inaccurate",
				furyctlBinVersion,
				furyctlConfSemVer,
			)
		}
	}

	if distroLocation == "" {
		distroLocation = fmt.Sprintf(DefaultBaseUrl, furyctlConfSemVer.String())
	}

	baseDst, err := os.MkdirTemp("", "furyctl-")
	if err != nil {
		return downloadResult{}, fmt.Errorf("%w: %v", ErrCreatingTempDir, err)
	}
	src := distroLocation
	dst := filepath.Join(baseDst, "data")

	logrus.Debugf("Downloading '%s' in '%s'", src, dst)

	if err := netx.NewGoGetterClient().Download(src, dst); err != nil {
		return downloadResult{}, fmt.Errorf("%w '%s': %v", ErrDownloadingFolder, src, err)
	}

	if !debug {
		defer osx.CleanupTempDir(filepath.Base(dst))

	}

	kfdPath := filepath.Join(dst, "kfd.yaml")
	kfdManifest, err := yaml.FromFileV3[Manifest](kfdPath)
	if err != nil {
		return downloadResult{}, err
	}

	if !semver.SamePatch(furyctlConfSemVer, kfdManifest.Version) {
		return downloadResult{}, fmt.Errorf(
			"versions mismatch: furyctl.yaml = '%s', furyctl binary = '%s'",
			furyctlConfSemVer.String(),
			kfdManifest.Version.String(),
		)
	}

	return downloadResult{
		RepoPath:       dst,
		MinimalConf:    minimalConf,
		DistroManifest: kfdManifest,
	}, nil
}

func GetSchemaPath(basePath string, conf FuryctlConfig) (string, error) {
	avp := strings.Split(conf.ApiVersion, "/")

	if len(avp) < 2 {
		return "", fmt.Errorf("invalid apiVersion: %s", conf.ApiVersion)
	}

	ns := strings.Replace(avp[0], ".sighup.io", "", 1)
	ver := avp[1]

	if conf.Kind == "" {
		return "", fmt.Errorf("kind is empty")
	}

	filename := fmt.Sprintf("%s-%s-%s.json", strings.ToLower(conf.Kind.String()), ns, ver)

	return filepath.Join(basePath, "schemas", filename), nil
}

func GetDefaultPath(basePath string) string {
	return filepath.Join(basePath, "furyctl-defaults.yaml")
}
