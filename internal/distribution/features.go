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
	FeatureClusterUpgrade     = Feature("ClusterUpgrade")
	FeatureTracingModule      = Feature("TracingModule")
	FeatureKubeconfigInSchema = Feature("KubeconfigInSchema")
	FeaturePlugins            = Feature("Plugins")
	FeatureYqSupport          = Feature("YqSupport")
	FeatureKubernetesLogTypes = Feature("KubernetesLogTypes")
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

	case FeatureYqSupport:
		return hasFeatureYQSupport(kfd)

	case FeatureKubernetesLogTypes:
		return hasFeatureKubernetesLogTypes(kfd)
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

func hasFeatureYQSupport(kfd config.KFD) bool {
	v, err := semver.NewVersion(kfd.Version)
	if err != nil {
		return false
	}

	v1253, err := semver.NewVersion("v1.25.3")
	if err != nil {
		return false
	}

	return v.GreaterThan(v1253)
}

func hasFeatureKubernetesLogTypes(kfd config.KFD) bool {
	v, err := semver.NewVersion(kfd.Version)
	if err != nil {
		return false
	}

	v1259, err := semver.NewVersion("v1.25.9")
	if err != nil {
		return false
	}

	v1264, err := semver.NewVersion("v1.26.4")
	if err != nil {
		return false
	}

	v1272, err := semver.NewVersion("v1.27.2")
	if err != nil {
		return false
	}

	v1260, err := semver.NewVersion("v1.26.0")
	if err != nil {
		return false
	}

	v1270, err := semver.NewVersion("v1.27.0")
	if err != nil {
		return false
	}

	return v.GreaterThan(v1259) && v.LessThan(v1260) ||
		v.GreaterThan(v1264) && v.LessThan(v1270) ||
		v.GreaterThan(v1272)
}
