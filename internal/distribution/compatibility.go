// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//nolint:dupl // duplicated code is acceptable in this case
package distribution

import (
	"errors"
	"fmt"

	"github.com/sighupio/furyctl/internal/git"
	"github.com/sighupio/furyctl/internal/semver"
)

const (
	EKSClusterKind         = "EKSCluster"
	KFDDistributionKind    = "KFDDistribution"
	OnPremisesKind         = "OnPremises"
	MinSupportedKFDVersion = "v1.25.8"
)

var ErrUnsupportedKind = errors.New("unsupported kind")

type CompatibilityChecker interface {
	IsCompatible() bool
}

type CompatibilityCheck struct {
	distributionVersion string
}

// Check the minimal KDF version supported by furyctl.
func IsReleaseUnsupportedByFuryctl(ghRelease git.Release) bool {
	distributionVersion := ghRelease.TagName

	latestSupported, err := semver.NewVersion(MinSupportedKFDVersion)
	if err != nil {
		return true
	}

	currentVersion, err := semver.NewVersion(distributionVersion)
	if err != nil {
		return true
	}

	return currentVersion.LessThan(latestSupported)
}

func NewCompatibilityChecker(distributionVersion, kind string) (CompatibilityChecker, error) {
	switch kind {
	case EKSClusterKind:
		return NewEKSClusterCheck(distributionVersion), nil

	case KFDDistributionKind:
		return NewKFDDistributionCheck(distributionVersion), nil

	case OnPremisesKind:
		return NewOnPremisesCheck(distributionVersion), nil

	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedKind, kind)
	}
}

type EKSClusterCheck struct {
	CompatibilityCheck
}

func NewEKSClusterCheck(distributionVersion string) *EKSClusterCheck {
	return &EKSClusterCheck{
		CompatibilityCheck: CompatibilityCheck{distributionVersion: distributionVersion},
	}
}

func (c *EKSClusterCheck) IsCompatible() bool {
	currentVersion, err := semver.NewVersion(c.distributionVersion)
	if err != nil {
		return false
	}

	min125Version, err := semver.NewVersion("v1.25.6")
	if err != nil {
		return false
	}

	max125Version, err := semver.NewVersion("v1.25.10")
	if err != nil {
		return false
	}

	min126Version, err := semver.NewVersion("v1.26.0")
	if err != nil {
		return false
	}

	max126Version, err := semver.NewVersion("v1.26.6")
	if err != nil {
		return false
	}

	min12SevenVersion, err := semver.NewVersion("v1.27.0")
	if err != nil {
		return false
	}

	max12SevenVersion, err := semver.NewVersion("v1.27.9")
	if err != nil {
		return false
	}

	min12EightVersion, err := semver.NewVersion("v1.28.0")
	if err != nil {
		return false
	}

	max12EightVersion, err := semver.NewVersion("v1.28.6")
	if err != nil {
		return false
	}

	min12NineVersion, err := semver.NewVersion("v1.29.0")
	if err != nil {
		return false
	}

	max12NineVersion, err := semver.NewVersion("v1.29.6")
	if err != nil {
		return false
	}

	min130Version, err := semver.NewVersion("v1.30.0")
	if err != nil {
		return false
	}

	max130Version, err := semver.NewVersion("v1.30.1")
	if err != nil {
		return false
	}

	min131Version, err := semver.NewVersion("v1.31.0")
	if err != nil {
		return false
	}

	max131Version, err := semver.NewVersion("v1.31.0")
	if err != nil {
		return false
	}

	return (currentVersion.GreaterThanOrEqual(min125Version) && currentVersion.LessThanOrEqual(max125Version)) ||
		(currentVersion.GreaterThanOrEqual(min126Version) && currentVersion.LessThanOrEqual(max126Version)) ||
		(currentVersion.GreaterThanOrEqual(min12SevenVersion) && currentVersion.LessThanOrEqual(max12SevenVersion)) ||
		(currentVersion.GreaterThanOrEqual(min12EightVersion)) && currentVersion.LessThanOrEqual(max12EightVersion) ||
		(currentVersion.GreaterThanOrEqual(min12NineVersion)) && currentVersion.LessThanOrEqual(max12NineVersion) ||
		(currentVersion.GreaterThanOrEqual(min130Version)) && currentVersion.LessThanOrEqual(max130Version) ||
		(currentVersion.GreaterThanOrEqual(min131Version)) && currentVersion.LessThanOrEqual(max131Version)
}

type KFDDistributionCheck struct {
	CompatibilityCheck
}

func NewKFDDistributionCheck(distributionVersion string) *KFDDistributionCheck {
	return &KFDDistributionCheck{
		CompatibilityCheck: CompatibilityCheck{distributionVersion: distributionVersion},
	}
}

func (c *KFDDistributionCheck) IsCompatible() bool {
	currentVersion, err := semver.NewVersion(c.distributionVersion)
	if err != nil {
		return false
	}

	min125Version, err := semver.NewVersion("v1.25.6")
	if err != nil {
		return false
	}

	max125Version, err := semver.NewVersion("v1.25.10")
	if err != nil {
		return false
	}

	min126Version, err := semver.NewVersion("v1.26.0")
	if err != nil {
		return false
	}

	max126Version, err := semver.NewVersion("v1.26.6")
	if err != nil {
		return false
	}

	min12SevenVersion, err := semver.NewVersion("v1.27.0")
	if err != nil {
		return false
	}

	max12SevenVersion, err := semver.NewVersion("v1.27.9")
	if err != nil {
		return false
	}

	min12EightVersion, err := semver.NewVersion("v1.28.0")
	if err != nil {
		return false
	}

	max12EightVersion, err := semver.NewVersion("v1.28.6")
	if err != nil {
		return false
	}

	min12NineVersion, err := semver.NewVersion("v1.29.0")
	if err != nil {
		return false
	}

	max12NineVersion, err := semver.NewVersion("v1.29.6")
	if err != nil {
		return false
	}

	min130Version, err := semver.NewVersion("v1.30.0")
	if err != nil {
		return false
	}

	max130Version, err := semver.NewVersion("v1.30.1")
	if err != nil {
		return false
	}

	min131Version, err := semver.NewVersion("v1.31.0")
	if err != nil {
		return false
	}

	max131Version, err := semver.NewVersion("v1.31.0")
	if err != nil {
		return false
	}

	return (currentVersion.GreaterThanOrEqual(min125Version) && currentVersion.LessThanOrEqual(max125Version)) ||
		(currentVersion.GreaterThanOrEqual(min126Version) && currentVersion.LessThanOrEqual(max126Version)) ||
		(currentVersion.GreaterThanOrEqual(min12SevenVersion) && currentVersion.LessThanOrEqual(max12SevenVersion)) ||
		(currentVersion.GreaterThanOrEqual(min12EightVersion)) && currentVersion.LessThanOrEqual(max12EightVersion) ||
		(currentVersion.GreaterThanOrEqual(min12NineVersion)) && currentVersion.LessThanOrEqual(max12NineVersion) ||
		(currentVersion.GreaterThanOrEqual(min130Version)) && currentVersion.LessThanOrEqual(max130Version) ||
		(currentVersion.GreaterThanOrEqual(min131Version)) &&
			currentVersion.LessThanOrEqual(max131Version)
}

type OnPremisesCheck struct {
	CompatibilityCheck
}

func NewOnPremisesCheck(distributionVersion string) *OnPremisesCheck {
	return &OnPremisesCheck{
		CompatibilityCheck: CompatibilityCheck{distributionVersion: distributionVersion},
	}
}

func (c *OnPremisesCheck) IsCompatible() bool {
	currentVersion, err := semver.NewVersion(c.distributionVersion)
	if err != nil {
		return false
	}

	min125Version, err := semver.NewVersion("v1.25.8")
	if err != nil {
		return false
	}

	max125Version, err := semver.NewVersion("v1.25.10")
	if err != nil {
		return false
	}

	min126Version, err := semver.NewVersion("v1.26.2")
	if err != nil {
		return false
	}

	max126Version, err := semver.NewVersion("v1.26.6")
	if err != nil {
		return false
	}

	min12SevenVersion, err := semver.NewVersion("v1.27.0")
	if err != nil {
		return false
	}

	max12SevenVersion, err := semver.NewVersion("v1.27.9")
	if err != nil {
		return false
	}

	min12EightVersion, err := semver.NewVersion("v1.28.0")
	if err != nil {
		return false
	}

	max12EightVersion, err := semver.NewVersion("v1.28.6")
	if err != nil {
		return false
	}

	min12NineVersion, err := semver.NewVersion("v1.29.0")
	if err != nil {
		return false
	}

	max12NineVersion, err := semver.NewVersion("v1.29.6")
	if err != nil {
		return false
	}

	min130Version, err := semver.NewVersion("v1.30.0")
	if err != nil {
		return false
	}

	max130Version, err := semver.NewVersion("v1.30.1")
	if err != nil {
		return false
	}

	min131Version, err := semver.NewVersion("v1.31.0")
	if err != nil {
		return false
	}

	max131Version, err := semver.NewVersion("v1.31.0")
	if err != nil {
		return false
	}

	return (currentVersion.GreaterThanOrEqual(min125Version) && currentVersion.LessThanOrEqual(max125Version)) ||
		(currentVersion.GreaterThanOrEqual(min126Version) && currentVersion.LessThanOrEqual(max126Version)) ||
		(currentVersion.GreaterThanOrEqual(min12SevenVersion) && currentVersion.LessThanOrEqual(max12SevenVersion)) ||
		(currentVersion.GreaterThanOrEqual(min12EightVersion)) && currentVersion.LessThanOrEqual(max12EightVersion) ||
		(currentVersion.GreaterThanOrEqual(min12NineVersion)) && currentVersion.LessThanOrEqual(max12NineVersion) ||
		(currentVersion.GreaterThanOrEqual(min130Version)) && currentVersion.LessThanOrEqual(max130Version) ||
		(currentVersion.GreaterThanOrEqual(min131Version)) &&
			currentVersion.LessThanOrEqual(max131Version)
}
