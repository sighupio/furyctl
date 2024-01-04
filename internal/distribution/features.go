// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package distribution

import (
	"errors"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/furyctl/internal/semver"
)

type Feature string

var ErrPluginsFeatureNotSupported = errors.New("plugins feature not supported")

const (
	FeatureClusterUpgrade = Feature("ClusterUpgrade")
	FeatureTracingModule  = Feature("TracingModule")
	// REMOVE THIS FEATURE AFTER v1.25 EOL.
	FeatureKubeconfigInSchema = Feature("KubeconfigInSchema")
	FeaturePlugins            = Feature("Plugins")
)

func HasFeature(kfd config.KFD, name Feature) bool {
	switch name {
	case FeatureClusterUpgrade:
		return hasFeatureClusterUpgrade(kfd)

	case FeatureKubeconfigInSchema:
		return hasFeatureKubeconfigInSchema(kfd)

	case FeatureTracingModule:
		return hasFeatureTracingModule(kfd)

	case FeaturePlugins:
		return hasFeaturePlugins(kfd)
	}

	return false
}

func hasFeatureClusterUpgrade(kfd config.KFD) bool {
	v1, err := semver.NewVersion(kfd.Version)
	if err != nil {
		return false
	}

	v125, err := semver.NewVersion("v1.25.7")
	if err != nil {
		return false
	}

	return v1.GreaterThanOrEqual(v125)
}

func hasFeatureKubeconfigInSchema(kfd config.KFD) bool {
	v1, err := semver.NewVersion(kfd.Version)
	if err != nil {
		return false
	}

	v1259, err := semver.NewVersion("v1.25.9")
	if err != nil {
		return false
	}

	v1260, err := semver.NewVersion("v1.26.0")
	if err != nil {
		return false
	}

	v1264, err := semver.NewVersion("v1.26.4")
	if err != nil {
		return false
	}

	return (v1.GreaterThanOrEqual(v1259) && v1.LessThan(v1260)) ||
		v1.GreaterThanOrEqual(v1264)
}

func hasFeatureTracingModule(kfd config.KFD) bool {
	v1, err := semver.NewVersion(kfd.Version)
	if err != nil {
		return false
	}

	v1264, err := semver.NewVersion("v1.26.4")
	if err != nil {
		return false
	}

	return v1.GreaterThanOrEqual(v1264)
}

func hasFeaturePlugins(kfd config.KFD) bool {
	v1, err := semver.NewVersion(kfd.Version)
	if err != nil {
		return false
	}

	v1258, err := semver.NewVersion("v1.25.8")
	if err != nil {
		return false
	}

	v1260, err := semver.NewVersion("v1.26.0")
	if err != nil {
		return false
	}

	v1262, err := semver.NewVersion("v1.26.2")
	if err != nil {
		return false
	}

	return v1.GreaterThanOrEqual(v1258) && v1.LessThan(v1260) ||
		v1.GreaterThanOrEqual(v1262)
}
