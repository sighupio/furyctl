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
			Object: git.TagCommit{SHA: "20", URL: "https://..."},
		}, {
			Ref:    "v1.22.0",
			Object: git.TagCommit{SHA: "22", URL: "https://..."},
		}, {
			Ref:    "v1.23.0",
			Object: git.TagCommit{SHA: "23", URL: "https://..."},
		}, {
			Ref:    "v1.24.0",
			Object: git.TagCommit{SHA: "27", URL: "https://..."},
		}, {
			Ref:    "v1.28.0",
			Object: git.TagCommit{SHA: "28", URL: "https://..."},
		}, {
			Ref:    "v1.29.0",
			Object: git.TagCommit{SHA: "29", URL: "https://..."},
		}, {
			Ref:    "v1.30.0",
			Object: git.TagCommit{SHA: "30", URL: "https://..."},
		}, {
			Ref:    "v1.31.0",
			Object: git.TagCommit{SHA: "31", URL: "https://..."},
		}},
		map[string]git.Commit{
			"31": {Tagger: &git.CommitAuthor{Name: "John Doe", Email: "john@example.com", Date: "2023-10-06T14:16:00Z"}},
			"30": {Author: &git.CommitAuthor{Name: "John Doe", Email: "john@example.com", Date: "2023-10-06T14:16:00Z"}},
			"29": {Tagger: &git.CommitAuthor{Name: "John Doe", Email: "john@example.com", Date: "2023-10-06T14:16:00Z"}},
			"28": {Author: &git.CommitAuthor{Name: "John Doe", Email: "john@example.com", Date: "2023-10-06T14:16:00Z"}},
			"27": {Author: &git.CommitAuthor{Name: "John Doe", Email: "john@example.com", Date: "2023-10-06T14:16:00Z"}},
			"23": {Author: &git.CommitAuthor{Name: "John Doe", Email: "john@example.com", Date: "2023-10-06T14:16:00Z"}},
			"22": {Author: &git.CommitAuthor{Name: "John Doe", Email: "john@example.com", Date: "2023-10-06T14:16:00Z"}},
			"20": {Author: &git.CommitAuthor{Name: "John Doe", Email: "john@example.com", Date: "2023-10-06T14:16:00Z"}},
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
