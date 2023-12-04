package distribution

import (
	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/furyctl/internal/semver"
)

type Feature string

const (
	FeatureClusterUpgrade     = Feature("ClusterUpgrade")
	FeatureKubeconfigInSchema = Feature("KubeconfigInSchema")
)

func HasFeature(kfd config.KFD, name Feature) bool {
	switch name {
	case FeatureClusterUpgrade:
		return hasFeatureClusterUpgrade(kfd)

	case FeatureKubeconfigInSchema:
		return hasFeatureKubeconfigFlag(kfd)
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

func hasFeatureKubeconfigFlag(kfd config.KFD) bool {
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
