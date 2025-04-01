// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package distribution

import (
	"errors"
	"fmt"

	"github.com/Al-Pragliola/go-version"

	"github.com/sighupio/furyctl/internal/git"
	"github.com/sighupio/furyctl/internal/semver"
)

const (
	EKSClusterKind         = "EKSCluster"
	KFDDistributionKind    = "KFDDistribution"
	OnPremisesKind         = "OnPremises"
	MinSupportedKFDVersion = "v1.25.8"
)

// VersionRange represents a min-max version range
type VersionRange struct {
	Min string
	Max string
}

// Compatible version ranges for different distribution types
var (
	// EKSCompatibleRanges defines version ranges compatible with both EKS and KFD
	EKSCompatibleRanges = []VersionRange{
		{"v1.25.6", "v1.25.10"},
		{"v1.26.0", "v1.26.6"},
		{"v1.27.0", "v1.27.9"},
		{"v1.28.0", "v1.28.6"},
		{"v1.29.0", "v1.29.7"},
		{"v1.30.0", "v1.30.2"},
		{"v1.31.0", "v1.31.1"},
		{"v1.32.0", "v1.32.0"},
	}

	// KFDCompatibleRanges defines version ranges compatible with both EKS and KFD
	KFDCompatibleRanges = []VersionRange{
		{"v1.25.6", "v1.25.10"},
		{"v1.26.0", "v1.26.6"},
		{"v1.27.0", "v1.27.9"},
		{"v1.28.0", "v1.28.6"},
		{"v1.29.0", "v1.29.7"},
		{"v1.30.0", "v1.30.2"},
		{"v1.31.0", "v1.31.1"},
		{"v1.32.0", "v1.32.0"},
	}

	// OnPremisesCompatibleRanges defines version ranges compatible with OnPremises
	OnPremisesCompatibleRanges = []VersionRange{
		{"v1.25.8", "v1.25.10"},
		{"v1.26.2", "v1.26.6"},
		{"v1.27.0", "v1.27.9"},
		{"v1.28.0", "v1.28.6"},
		{"v1.29.0", "v1.29.7"},
		{"v1.30.0", "v1.30.2"},
		{"v1.31.0", "v1.31.1"},
		{"v1.32.0", "v1.32.0"},
	}
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
	// Parse the current version
	currentVersion, err := semver.NewVersion(c.distributionVersion)
	if err != nil {
		return false
	}

	return isVersionInAnyRange(currentVersion, EKSCompatibleRanges)
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
	// Parse the current version
	currentVersion, err := semver.NewVersion(c.distributionVersion)
	if err != nil {
		return false
	}

	return isVersionInAnyRange(currentVersion, KFDCompatibleRanges)
}

// isVersionInAnyRange checks if the given version is within any of the specified version ranges
func isVersionInAnyRange(currentVersion *version.Version, compatibleRanges []VersionRange) bool {
	// Helper function to safely create a version
	newVersion := func(v string) (*version.Version, bool) {
		version, err := semver.NewVersion(v)
		return version, err == nil
	}

	// Check if current version is within any of the compatible ranges
	for _, r := range compatibleRanges {
		minVersion, minOk := newVersion(r.Min)
		maxVersion, maxOk := newVersion(r.Max)

		if !minOk || !maxOk {
			continue // Skip this range if we can't parse the versions
		}

		if currentVersion.GreaterThanOrEqual(minVersion) && currentVersion.LessThanOrEqual(maxVersion) {
			return true
		}
	}

	return false
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
	// Parse the current version
	currentVersion, err := semver.NewVersion(c.distributionVersion)
	if err != nil {
		return false
	}

	return isVersionInAnyRange(currentVersion, OnPremisesCompatibleRanges)
}
