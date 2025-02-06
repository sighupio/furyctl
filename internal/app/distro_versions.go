// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/Al-Pragliola/go-version"
	"github.com/sirupsen/logrus"

	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/git"
)

// DistroRelease holds information about a distribution release.
type DistroRelease struct {
	Version        version.Version
	Sha            string
	Date           time.Time
	FuryctlSupport FuryctlSupported
}

// FuryctlSupported holds boolean flags for supported distributions.
type FuryctlSupported struct {
	EKSCluster      bool
	KFDDistribution bool
	OnPremises      bool
}

// GetSupportedDistroVersions retrieves distro releases filtering out unsupported versions.
func GetSupportedDistroVersions(ghClient git.RepoClient) ([]DistroRelease, error) {
	releases := []DistroRelease{}

	// Fetch all tags from the GitHub API.
	tags, err := ghClient.GetTags()
	if err != nil {
		return releases, fmt.Errorf("error getting tags from github: %w", err)
	}

	// Get the latest distro version from the tag list.
	latestRelease, err := getLatestDistroVersion(ghClient, tags)
	if err != nil {
		return releases, fmt.Errorf("error getting latest distro version: %w", err)
	}

	// Calculate the latest supported version based on the latest release.
	latestSupportedVersion := GetLatestSupportedVersion(latestRelease.Version)

	// Loop over all tags except the final element and only keep supported ones.
	for _, tag := range tags {
		v, err := VersionFromRef(tag.Ref)
		if err != nil || v.LessThan(&latestSupportedVersion) || v.Prerelease() != "" {
			continue
		}

		release, err := newDistroRelease(ghClient, tag)
		if err != nil {
			// Skip tags that cannot be parsed or processed.
			continue
		}

		releases = append(releases, release)
	}

	slices.Reverse(releases)

	return releases, nil
}

const previousSupportedVersions = 2

// GetLatestSupportedVersion returns the supported version based on the second segment of the version.
func GetLatestSupportedVersion(v version.Version) version.Version {
	// Generate a version string using the second segment from the provided version.
	versionStr := fmt.Sprintf("1.%d.0", v.Segments()[1]-previousSupportedVersions)

	supportedVersion, err := version.NewSemver(versionStr)
	if err != nil {
		return version.Version{}
	}

	return *supportedVersion
}

var (
	ErrLatestDistroVersionNotFound = errors.New("latest distro not found")
	ErrInvalidVersion              = errors.New("invalid version")
)

// GetLatestDistroVersion iterates backward over tags to return the latest valid distro release.
func getLatestDistroVersion(ghClient git.RepoClient, tags []git.Tag) (DistroRelease, error) {
	// Iterate from last to first using slices.Backward.
	for _, tag := range slices.Backward(tags) {
		version, err := VersionFromRef(tag.Ref)
		if err != nil {
			continue
		}
		// Skip prerelease versions.
		if version.Prerelease() != "" {
			continue
		}

		return newDistroRelease(ghClient, tag)
	}

	return DistroRelease{}, ErrLatestDistroVersionNotFound
}

// NewDistroRelease creates a DistroRelease from a Tag, fetching its commit details.
func newDistroRelease(ghClient git.RepoClient, tag git.Tag) (DistroRelease, error) {
	var release DistroRelease

	// Parse version from tag reference.
	version, err := VersionFromRef(tag.Ref)
	if err != nil {
		logrus.Debug(err)

		return release, fmt.Errorf("invalid version: %w", err)
	}

	// Fetch the commit information using the SHA.
	commit, err := ghClient.GetCommit(tag.Object.SHA)
	if err != nil {
		logrus.Error(err)

		return release, fmt.Errorf("error getting commit: %w", err)
	}

	// Parse the commit date.
	var commitDate time.Time
	if commit.Author != nil {
		commitDate, err = time.Parse(time.RFC3339, commit.Author.Date)
		if err != nil {
			commitDate = time.Time{}
		}
	} else if commit.Tagger != nil {
		commitDate, err = time.Parse(time.RFC3339, commit.Tagger.Date)
		if err != nil {
			commitDate = time.Time{}
		}
	}

	// Build the release struct.
	release = DistroRelease{
		Version:        version,
		Sha:            tag.Object.SHA,
		Date:           commitDate,
		FuryctlSupport: GetFuryctlSupport(version),
	}

	return release, nil
}

// GetFuryctlSupport checks for compatibility with various distributions.
func GetFuryctlSupport(version version.Version) FuryctlSupported {
	eks, errEKS := distribution.NewCompatibilityChecker(version.String(), distribution.EKSClusterKind)
	kfd, errKFD := distribution.NewCompatibilityChecker(version.String(), distribution.KFDDistributionKind)
	onprem, errOnPrem := distribution.NewCompatibilityChecker(version.String(), distribution.OnPremisesKind)

	// Helper function to interpret compatibility result.
	isCompatible := func(checker distribution.CompatibilityChecker, err error) bool {
		if err != nil {
			return false
		}

		return checker.IsCompatible()
	}

	return FuryctlSupported{
		EKSCluster:      isCompatible(eks, errEKS),
		KFDDistribution: isCompatible(kfd, errKFD),
		OnPremises:      isCompatible(onprem, errOnPrem),
	}
}

// VersionFromRef converts a tag ref string to a semver version.
// Expected format: "refs/tags/v1.2.3".
func VersionFromRef(ref string) (version.Version, error) {
	var v version.Version
	// Remove the "refs/tags/" prefix.
	versionStr := strings.ReplaceAll(ref, "refs/tags/", "")

	if !strings.HasPrefix(versionStr, "v") {
		return v, ErrInvalidVersion
	}

	// Remove the "v" prefix to isolate the version number.
	versionStr = versionStr[1:]

	ver, err := version.NewSemver(versionStr)
	if err != nil {
		return v, ErrInvalidVersion
	}

	return *ver, nil
}
