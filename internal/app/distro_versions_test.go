package app

import (
	"testing"

	"github.com/Al-Pragliola/go-version"
	"github.com/sighupio/furyctl/internal/git"
	"github.com/sighupio/furyctl/mocks"
	"github.com/stretchr/testify/assert"
)

func TestGetSupportedDistroVersions(t *testing.T) {
	// Mock GitHub client
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
			"31": {Author: git.CommitAuthor{Name: "John Doe", Email: "john@example.com", Date: "2023-10-06T14:16:00Z"}},
			"30": {Author: git.CommitAuthor{Name: "John Doe", Email: "john@example.com", Date: "2023-10-06T14:16:00Z"}},
			"29": {Author: git.CommitAuthor{Name: "John Doe", Email: "john@example.com", Date: "2023-10-06T14:16:00Z"}},
			"28": {Author: git.CommitAuthor{Name: "John Doe", Email: "john@example.com", Date: "2023-10-06T14:16:00Z"}},
			"27": {Author: git.CommitAuthor{Name: "John Doe", Email: "john@example.com", Date: "2023-10-06T14:16:00Z"}},
			"23": {Author: git.CommitAuthor{Name: "John Doe", Email: "john@example.com", Date: "2023-10-06T14:16:00Z"}},
			"22": {Author: git.CommitAuthor{Name: "John Doe", Email: "john@example.com", Date: "2023-10-06T14:16:00Z"}},
			"20": {Author: git.CommitAuthor{Name: "John Doe", Email: "john@example.com", Date: "2023-10-06T14:16:00Z"}},
		},
	)

	// Call the function being tested
	releases, err := GetSupportedDistroVersions(mockGhClient)

	// Assert results
	assert.NoError(t, err)
	assert.Equal(t, 3, len(releases))
	assert.Equal(t, "1.29.0", releases[0].Version.String())
}

func TestGetLatestSupportedVersion(t *testing.T) {
	// Test case for GetLatestSupportedVersion
	v, _ := version.NewSemver("1.31.0")
	supportedV := GetLatestSupportedVersion(*v)
	assert.Equal(t, "1.29.0", supportedV.String())
}

func TestVersionFromRef(t *testing.T) {
	// Test case for VersionFromRef
	ref := "refs/tags/v1.2.3-abcXXX"
	v, err := VersionFromRef(ref)
	assert.NoError(t, err)
	assert.Equal(t, "1.2.3-abcXXX", v.String())
}
