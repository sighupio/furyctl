// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package distribution

import (
	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/furyctl/internal/semver"
)

type Feature string

const (
	FeatureClusterUpgrade = Feature("ClusterUpgrade")
	// REMOVE THIS FEATURE AFTER v1.25 EOL.
	FeatureKubeconfigInSchema = Feature("KubeconfigInSchema")
)

func HasFeature(kfd config.KFD, name Feature) bool {
	switch name {
	case FeatureClusterUpgrade:
		return hasFeatureClusterUpgrade(kfd)

	case FeatureKubeconfigInSchema:
		return hasFeatureKubeconfigInSchema(kfd)
	}

	return false
}

func hasFeatureClusterUpgrade(kfd config.KFD) bool {
	v1, err := semver.NewVersion(kfd.Version)
	if err != nil {
		return false
	}

	v2, err := semver.NewVersion("v1.26.0")
	if err != nil {
		return false
	}

	return v1.GreaterThanOrEqual(v2)
}

func hasFeatureKubeconfigInSchema(kfd config.KFD) bool {
	v1, err := semver.NewVersion(kfd.Version)
	if err != nil {
		return false
	}

	v2, err := semver.NewVersion("v1.26.0")
	if err != nil {
		return false
	}

	return v1.GreaterThanOrEqual(v2)
}
