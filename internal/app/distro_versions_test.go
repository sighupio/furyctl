// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app_test

import (
	"testing"

	"github.com/Al-Pragliola/go-version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/app"
	"github.com/sighupio/furyctl/internal/git"
	"github.com/sighupio/furyctl/mocks"
)

func TestGetSupportedDistroVersions(t *testing.T) {
	t.Parallel()
	// Mock GitHub client.
	mockGhClient := mocks.NewMockGitHubClient(
		[]git.Tag{{
			Ref:    "v1.20.0",
			Object: git.TagRef{SHA: "20", URL: "https://.../20"},
		}, {
			Ref:    "v1.22.0",
			Object: git.TagRef{SHA: "22", URL: "https://.../22"},
		}, {
			Ref:    "v1.23.0",
			Object: git.TagRef{SHA: "23", URL: "https://.../23"},
		}, {
			Ref:    "v1.24.0",
			Object: git.TagRef{SHA: "27", URL: "https://.../24"},
		}, {
			Ref:    "v1.28.0",
			Object: git.TagRef{SHA: "28", URL: "https://.../28"},
		}, {
			Ref:    "v1.29.0",
			Object: git.TagRef{SHA: "29", URL: "https://.../29"},
		}, {
			Ref:    "v1.30.0",
			Object: git.TagRef{SHA: "30", URL: "https://.../30"},
		}, {
			Ref:    "v1.31.0",
			Object: git.TagRef{SHA: "31", URL: "https://.../31"},
		}},
		map[string]git.ObjectInfo{
			"https://.../31": {Tagger: &git.Author{Name: "John Doe", Email: "john@example.com", Date: "2023-10-06T14:16:00Z"}},
			"https://.../30": {Author: &git.Author{Name: "John Doe", Email: "john@example.com", Date: "2023-10-06T14:16:00Z"}},
			"https://.../29": {Tagger: &git.Author{Name: "John Doe", Email: "john@example.com", Date: "2023-10-06T14:16:00Z"}},
			"https://.../28": {Author: &git.Author{Name: "John Doe", Email: "john@example.com", Date: "2023-10-06T14:16:00Z"}},
			"https://.../27": {Author: &git.Author{Name: "John Doe", Email: "john@example.com", Date: "2023-10-06T14:16:00Z"}},
			"https://.../23": {Author: &git.Author{Name: "John Doe", Email: "john@example.com", Date: "2023-10-06T14:16:00Z"}},
			"https://.../22": {Author: &git.Author{Name: "John Doe", Email: "john@example.com", Date: "2023-10-06T14:16:00Z"}},
			"https://.../20": {Author: &git.Author{Name: "John Doe", Email: "john@example.com", Date: "2023-10-06T14:16:00Z"}},
		},
	)

	// Call the function being tested.
	releases, err := app.GetSupportedDistroVersions(mockGhClient)

	// Assert results.
	require.NoError(t, err)
	assert.Len(t, releases, 3)
	assert.Equal(t, "1.31.0", releases[0].Version.String())
}

func TestGetLatestSupportedVersion(t *testing.T) {
	t.Parallel()
	// Test case for GetLatestSupportedVersion.
	v, _ := version.NewSemver("1.31.0")
	supportedV := app.GetLatestSupportedVersion(*v)
	assert.Equal(t, "1.29.0", supportedV.String())
}

func TestVersionFromRef(t *testing.T) {
	t.Parallel()
	// Test case for VersionFromRef.
	ref := "refs/tags/v1.2.3-abcXXX"
	v, err := app.VersionFromRef(ref)
	require.NoError(t, err)
	assert.Equal(t, "1.2.3-abcXXX", v.String())
}
