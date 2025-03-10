// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package distribution

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Al-Pragliola/go-version"
	"github.com/sirupsen/logrus"
	"slices"

	"github.com/sighupio/furyctl/internal/git"
)

// KFDRelease holds information about a distribution release.
type KFDRelease struct {
	Version     version.Version
	Date        time.Time
	Support     map[string]bool
	Recommended bool
}

// Check if current KFD version is a release or a prerelease.
func IsNotRelease(ghRelease git.Release) bool {
	v, err := VersionFromString(ghRelease.TagName)

	if err != nil || v.Prerelease() != "" {
		return true
	}
	return false
}

// GetSupportedVersions retrieves all distro releases filtering out prereleases and invalid versions.
func GetSupportedVersions(ghClient git.RepoClient) ([]KFDRelease, error) {
	releases := []KFDRelease{}

	// Fetch all releases from the GitHub API.
	ghReleases, err := ghClient.GetReleases()
	if err != nil {
		return releases, fmt.Errorf("error getting releases from GitHub: %w", err)
	}

	sort.Slice(ghReleases, func(i, j int) bool {
		iRelease := ghReleases[i]
		jRelease := ghReleases[j]

		iVersion, err := VersionFromString(iRelease.TagName)
		if err != nil {
			return false
		}

		jVersion, err := VersionFromString(jRelease.TagName)
		if err != nil {
			return true
		}

		return jVersion.LessThan(&iVersion)
	})

	ghReleases = slices.DeleteFunc(ghReleases, IsReleaseUnsupportedByFuryctl)

	ghReleases = slices.DeleteFunc(ghReleases, IsNotRelease)

	// Loop over releases and skip invalid versions.
	for _, ghRelease := range ghReleases {
		release, err := newKFDRelease(ghRelease)
		if err != nil {
			// Skip releases that cannot be parsed or processed.
			continue
		}

		releases = append(releases, release)
	}

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
	ErrLatestDistroVersionNotFound = errors.New("latest KFD version not found")
	ErrInvalidVersion              = errors.New("invalid version")
)

// GetLatestDistroVersion iterates over tags to return the latest valid distro release(not RC or prerelease).
func getLatestDistroVersion(ghReleases []git.Release) (KFDRelease, error) {
	for _, ghRelease := range ghReleases {
		if ghRelease.PreRelease {
			continue
		}

		version, err := VersionFromString(ghRelease.TagName)
		if err != nil {
			continue
		}

		// Skip prerelease versions.
		if version.Prerelease() != "" {
			continue
		}

		return newKFDRelease(ghRelease)
	}

	return KFDRelease{}, ErrLatestDistroVersionNotFound
}

// GetRecommendedVersions returns the last 3 minor with the last patch.
func GetRecommendedVersions(releases []KFDRelease) {
	minorVersionsFound := make(map[int]bool)
	recommendedCounter := 0

	for i := range releases {
		segments := releases[i].Version.Segments()
		if len(segments) < 2 {
			continue
		}

		minor := segments[1]
		if !minorVersionsFound[minor] {
			minorVersionsFound[minor] = true
			releases[i].Recommended = true
			recommendedCounter++

			if recommendedCounter == 3 {
				return
			}
		}
	}
}

// NewKFDRelease creates a KFDRelease from a Release.
func newKFDRelease(ghRelease git.Release) (KFDRelease, error) {
	var release KFDRelease

	// Parse version from Release tag name.
	version, err := VersionFromString(ghRelease.TagName)
	if err != nil {
		logrus.Debug(err)

		return release, fmt.Errorf("invalid version: %w", err)
	}

	// Build the release struct.
	release = KFDRelease{
		Version: version,
		Date:    ghRelease.CreatedAt,
		Support: GetSupport(version),
	}

	return release, nil
}

// GetSupport checks for compatibility with various distributions.
func GetSupport(version version.Version) map[string]bool {
	eks, errEKS := NewCompatibilityChecker(version.String(), EKSClusterKind)
	kfd, errKFD := NewCompatibilityChecker(version.String(), KFDDistributionKind)
	onprem, errOnPrem := NewCompatibilityChecker(version.String(), OnPremisesKind)

	// Helper function to interpret compatibility result.
	isCompatible := func(checker CompatibilityChecker, err error) bool {
		if err != nil {
			return false
		}

		return checker.IsCompatible()
	}

	support := make(map[string]bool)
	support[EKSClusterKind] = isCompatible(eks, errEKS)
	support[KFDDistributionKind] = isCompatible(kfd, errKFD)
	support[OnPremisesKind] = isCompatible(onprem, errOnPrem)

	return support
}

// VersionFromString converts a tag ref string to a semver version.
// Expected format: "refs/tags/v1.2.3".
func VersionFromString(ref string) (version.Version, error) {
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
