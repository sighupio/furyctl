// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package distribution

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Al-Pragliola/go-version"

	"github.com/sighupio/furyctl/internal/git"
	"github.com/sighupio/furyctl/internal/semver"
)

const (
	EKSClusterKind         = "EKSCluster"
	KFDDistributionKind    = "KFDDistribution"
	OnPremisesKind         = "OnPremises"
	ImmutableKind          = "Immutable"
	MinSupportedKFDVersion = "v1.25.8"
)

// VersionRange represents a min-max version range.
type VersionRange struct {
	Min string
	Max string
}

// ConfigKinds returns the list of supported configuration kinds.
func ConfigKinds() []string {
	// We should get this list from the supported APIs instead of hardcoding it,
	// but AFAIK we don't have a way to do that yet.
	return []string{
		EKSClusterKind,
		KFDDistributionKind,
		OnPremisesKind,
		ImmutableKind,
	}
}

// ValidateConfigKind checks if the given kind is supported and returns the normalised value for the Kind and an error.
func ValidateConfigKind(kind string) (string, error) {
	for _, k := range ConfigKinds() {
		if strings.EqualFold(k, kind) {
			return k, nil
		}
	}

	return "", fmt.Errorf("\"%s\" %w", kind, ErrUnsupportedKind)
}

// getEKSCompatibleRanges returns version ranges compatible with EKS.
func getEKSCompatibleRanges() []VersionRange {
	return []VersionRange{
		{"v1.25.6", "v1.25.10"},
		{"v1.26.0", "v1.26.6"},
		{"v1.27.0", "v1.27.9"},
		{"v1.28.0", "v1.28.6"},
		{"v1.29.0", "v1.29.7"},
		{"v1.30.0", "v1.30.2"},
		{"v1.31.0", "v1.31.2"},
		{"v1.32.0", "v1.32.1"},
		{"v1.33.0", "v1.33.1"},
		{"v1.34.0", "v1.34.0"},
	}
}

// getKFDCompatibleRanges returns version ranges compatible with KFD.
func getKFDCompatibleRanges() []VersionRange {
	return []VersionRange{
		{"v1.25.6", "v1.25.10"},
		{"v1.26.0", "v1.26.6"},
		{"v1.27.0", "v1.27.9"},
		{"v1.28.0", "v1.28.6"},
		{"v1.29.0", "v1.29.7"},
		{"v1.30.0", "v1.30.2"},
		{"v1.31.0", "v1.31.2"},
		{"v1.32.0", "v1.32.1"},
		{"v1.33.0", "v1.33.1"},
		{"v1.34.0", "v1.34.0"},
	}
}

func getImmutableCompatibleRanges() []VersionRange {
	return []VersionRange{
		{"v1.34.0", "v1.34.0"},
	}
}

// getOnPremisesCompatibleRanges returns version ranges compatible with OnPremises.
func getOnPremisesCompatibleRanges() []VersionRange {
	return []VersionRange{
		{"v1.25.8", "v1.25.10"},
		{"v1.26.2", "v1.26.6"},
		{"v1.27.0", "v1.27.9"},
		{"v1.28.0", "v1.28.6"},
		{"v1.29.0", "v1.29.7"},
		{"v1.30.0", "v1.30.2"},
		{"v1.31.0", "v1.31.2"},
		{"v1.32.0", "v1.32.1"},
		{"v1.33.0", "v1.33.1"},
		{"v1.34.0", "v1.34.0"},
	}
}

var ErrUnsupportedKind = errors.New("kind is not valid. Accepted values are " +
	strings.Join(ConfigKinds(), ", "))

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
	normalisedKind, err := ValidateConfigKind(kind)
	if err != nil {
		return nil, fmt.Errorf("\"%s\" %w", kind, ErrUnsupportedKind)
	}

	switch normalisedKind {
	case EKSClusterKind:
		return NewEKSClusterCheck(distributionVersion), nil

	case KFDDistributionKind:
		return NewKFDDistributionCheck(distributionVersion), nil

	case OnPremisesKind:
		return NewOnPremisesCheck(distributionVersion), nil

	case ImmutableKind:
		return NewImmutableCheck(distributionVersion), nil

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
	// Parse the current version.
	currentVersion, err := semver.NewVersion(c.distributionVersion)
	if err != nil {
		return false
	}

	return isVersionInAnyRange(currentVersion, getEKSCompatibleRanges())
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
	// Parse the current version.
	currentVersion, err := semver.NewVersion(c.distributionVersion)
	if err != nil {
		return false
	}

	return isVersionInAnyRange(currentVersion, getKFDCompatibleRanges())
}

// isVersionInAnyRange checks if the given version is within any of the specified version ranges.
func isVersionInAnyRange(currentVersion *version.Version, compatibleRanges []VersionRange) bool {
	// Helper function to safely create a version.
	newVersion := func(v string) (*version.Version, bool) {
		version, err := semver.NewVersion(v)

		return version, err == nil
	}

	// Check if current version is within any of the compatible ranges.
	for _, r := range compatibleRanges {
		minVersion, minOk := newVersion(r.Min)
		maxVersion, maxOk := newVersion(r.Max)

		if !minOk || !maxOk {
			continue // Skip this range if we can't parse the versions.
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

type ImmutableCheck struct {
	CompatibilityCheck
}

func NewImmutableCheck(distributionVersion string) *ImmutableCheck {
	return &ImmutableCheck{
		CompatibilityCheck: CompatibilityCheck{distributionVersion: distributionVersion},
	}
}

func (c *ImmutableCheck) IsCompatible() bool {
	// Parse the current version.
	currentVersion, err := semver.NewVersion(c.distributionVersion)
	if err != nil {
		return false
	}

	return isVersionInAnyRange(currentVersion, getImmutableCompatibleRanges())
}

func (c *OnPremisesCheck) IsCompatible() bool {
	// Parse the current version.
	currentVersion, err := semver.NewVersion(c.distributionVersion)
	if err != nil {
		return false
	}

	return isVersionInAnyRange(currentVersion, getOnPremisesCompatibleRanges())
}
