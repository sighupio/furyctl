// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package distribution_test

import (
	"testing"
	"time"

	"github.com/Al-Pragliola/go-version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/git"
	"github.com/sighupio/furyctl/mocks"
)

func TestGetSupportedDistroVersions(t *testing.T) {
	t.Parallel()
	// Mock GitHub client.
	mockGhClient := mocks.NewMockGitHubClient(
		[]git.Release{{
			TagName:     "v1.20.0",
			CreatedAt:   time.Date(2020, time.February, 6, 12, 30, 0, 0, time.UTC),
			PublishedAt: time.Date(2020, time.February, 7, 12, 30, 0, 0, time.UTC),
			PreRelease:  false,
		}, {
			TagName:     "v1.22.0",
			CreatedAt:   time.Date(2021, time.February, 6, 12, 30, 0, 0, time.UTC),
			PublishedAt: time.Date(2021, time.February, 7, 12, 30, 0, 0, time.UTC),
			PreRelease:  false,
		}, {
			TagName:     "v1.25.0",
			CreatedAt:   time.Date(2022, time.February, 6, 12, 30, 0, 0, time.UTC),
			PublishedAt: time.Date(2022, time.February, 7, 12, 30, 0, 0, time.UTC),
			PreRelease:  false,
		}, {
			TagName:     "v1.27.0",
			CreatedAt:   time.Date(2022, time.February, 6, 12, 30, 0, 0, time.UTC),
			PublishedAt: time.Date(2020, time.February, 7, 12, 30, 0, 0, time.UTC),
			PreRelease:  false,
		}, {
			TagName:     "v1.28.0",
			CreatedAt:   time.Date(2025, time.February, 6, 12, 30, 0, 0, time.UTC),
			PublishedAt: time.Date(2025, time.February, 7, 12, 30, 0, 0, time.UTC),
			PreRelease:  false,
		}, {
			TagName:     "v1.29.0",
			CreatedAt:   time.Date(2023, time.February, 6, 12, 30, 0, 0, time.UTC),
			PublishedAt: time.Date(2023, time.February, 7, 12, 30, 0, 0, time.UTC),
			PreRelease:  false,
		}, {
			TagName:     "v1.30.0",
			CreatedAt:   time.Date(2025, time.February, 6, 12, 30, 0, 0, time.UTC),
			PublishedAt: time.Date(2025, time.February, 7, 12, 30, 0, 0, time.UTC),
			PreRelease:  false,
		}, {
			TagName:     "v1.31.0",
			CreatedAt:   time.Date(2025, time.February, 6, 12, 30, 0, 0, time.UTC),
			PublishedAt: time.Date(2025, time.February, 7, 12, 30, 0, 0, time.UTC),
			PreRelease:  false,
		}, {
			TagName:     "v1.31.1",
			CreatedAt:   time.Date(2025, time.February, 6, 12, 30, 0, 0, time.UTC),
			PublishedAt: time.Date(2025, time.February, 7, 12, 30, 0, 0, time.UTC),
			PreRelease:  false,
		}},
	)

	// Call the function being tested.
	releases, err := distribution.GetSupportedVersions(mockGhClient)

	// Assert results.
	require.NoError(t, err)
	assert.Len(t, releases, 6)
	assert.Equal(t, "1.31.1", releases[0].Version.String())
	assert.Equal(t, "1.31.0", releases[1].Version.String())
	assert.Equal(t, "1.30.0", releases[2].Version.String())
}

func TestGetLatestSupportedVersion(t *testing.T) {
	t.Parallel()
	// Test case for GetLatestSupportedVersion.
	v, _ := version.NewSemver("1.31.0")
	supportedV := distribution.GetLatestSupportedVersion(*v)
	assert.Equal(t, "1.29.0", supportedV.String())
}

func TestVersionFromString(t *testing.T) {
	t.Parallel()
	// Test case for VersionFromString.
	ref := "v1.2.3-abcXXX"
	v, err := distribution.VersionFromString(ref)
	require.NoError(t, err)
	assert.Equal(t, "1.2.3-abcXXX", v.String())
}
