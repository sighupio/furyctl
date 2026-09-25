// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package upgradeanalysis

import (
	"fmt"

	"github.com/sighupio/furyctl/internal/apis/config"
	"github.com/sighupio/furyctl/internal/semver"
	dist "github.com/sighupio/furyctl/pkg/distribution"
)

// DownloadFetcher reads the KFD manifest of a distribution version by downloading that
// version of the distribution. Results are kept for the lifetime of the fetcher, since a
// chain visits the same version twice: as the target of one hop and the source of the next.
type DownloadFetcher struct {
	downloader *dist.Downloader
	cache      map[string]Distribution
}

func NewDownloadFetcher(downloader *dist.Downloader) *DownloadFetcher {
	return &DownloadFetcher{
		downloader: downloader,
		cache:      map[string]Distribution{},
	}
}

// Fetch returns the distribution of one version, downloading it when needed.
func (f *DownloadFetcher) Fetch(kind, version string) (Distribution, error) {
	if cached, hit := f.cache[version]; hit {
		return cached, nil
	}

	result, err := f.downloader.DoDownload("", config.Furyctl{
		Kind: kind,
		Spec: config.FuryctlSpec{DistributionVersion: semver.EnsurePrefix(version)},
	})
	if err != nil {
		return Distribution{}, fmt.Errorf("error downloading distribution %s: %w", version, err)
	}

	distribution := Distribution{Manifest: result.DistroManifest, Path: result.RepoPath}
	f.cache[version] = distribution

	return distribution, nil
}
