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

	"github.com/hashicorp/go-getter"
	"github.com/sirupsen/logrus"

	"github.com/sighupio/furyctl/internal/semver"
	"github.com/sighupio/furyctl/internal/yaml"
)

const DefaultBaseUrl = "https://git@github.com/sighupio/fury-distribution?ref=%s"

var (
	downloadProtocols = []string{"", "git::", "file::", "http::", "s3::", "gcs::", "mercurial::"}

	errDownloadOptionsExausted = errors.New("downloading options exausted")

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

	repoPath, err := DownloadDirectory(distroLocation)
	if err != nil {
		return downloadResult{}, err
	}
	if !debug {
		defer CleanupTempDir(filepath.Base(repoPath))
	}

	kfdPath := filepath.Join(repoPath, "kfd.yaml")
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
		RepoPath:       repoPath,
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

func DownloadDirectory(src string) (string, error) {
	baseDst, err := os.MkdirTemp("", "furyctl-")
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrCreatingTempDir, err)
	}

	dst := filepath.Join(baseDst, "data")

	logrus.Debugf("Downloading '%s' in '%s'", src, dst)

	if err := ClientGet(src, dst); err != nil {
		return "", fmt.Errorf("%w '%s': %v", ErrDownloadingFolder, src, err)
	}

	return dst, nil
}

func CleanupTempDir(dir string) {
	if err := os.RemoveAll(dir); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			logrus.Error(err)
		}
	}
}

// ClientGet tries a few different protocols to get the source file or directory.
func ClientGet(src, dst string) error {
	protocols := []string{""}
	if !UrlHasForcedProtocol(src) {
		protocols = downloadProtocols
	}

	for _, protocol := range protocols {
		fullSrc := fmt.Sprintf("%s%s", protocol, src)

		logrus.Debugf("Trying to download from: %s", fullSrc)

		client := &getter.Client{
			Src:  fullSrc,
			Dst:  dst,
			Mode: getter.ClientModeAny,
		}

		err := client.Get()
		if err == nil {
			return nil
		}

		logrus.Debug(err)
	}

	return errDownloadOptionsExausted
}

// UrlHasForcedProtocol checks if the url has a forced protocol as described in hashicorp/go-getter.
func UrlHasForcedProtocol(url string) bool {
	for _, dp := range downloadProtocols {
		if dp != "" && strings.HasPrefix(url, dp) {
			return true
		}
	}

	return false
}
