// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package get_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/cmd/get"
	"github.com/sighupio/furyctl/internal/app"
	"github.com/sighupio/furyctl/internal/git"
	"github.com/sighupio/furyctl/mocks"
)

func TestFormatDistroVersions(t *testing.T) {
	t.Parallel()

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
			"31": {Tagger: &git.CommitAuthor{Name: "John Doe", Email: "john@example.com", Date: "2024-10-06T14:16:00Z"}},
			"30": {Author: &git.CommitAuthor{Name: "John Doe", Email: "john@example.com", Date: "2023-10-06T14:16:00Z"}},
			"29": {Author: &git.CommitAuthor{Name: "John Doe", Email: "john@example.com", Date: "2022-10-06T14:16:00Z"}},
			"28": {Tagger: &git.CommitAuthor{Name: "John Doe", Email: "john@example.com", Date: "2020-10-06T14:16:00Z"}},
			"27": {Tagger: &git.CommitAuthor{Name: "John Doe", Email: "john@example.com", Date: "2019-10-06T14:16:00Z"}},
			"23": {Author: &git.CommitAuthor{Name: "John Doe", Email: "john@example.com", Date: "2018-10-06T14:16:00Z"}},
			"22": {Author: &git.CommitAuthor{Name: "John Doe", Email: "john@example.com", Date: "2023-10-06T14:16:00Z"}},
			"20": {Author: &git.CommitAuthor{Name: "John Doe", Email: "john@example.com", Date: "2023-10-06T14:16:00Z"}},
		},
	)

	releases, err := app.GetSupportedDistroVersions(mockGhClient)

	require.NoError(t, err)

	fmtString := get.FormatDistroVersions(releases)
	lines := strings.Split(fmtString, "\n")

	assert.Equal(t, "AVAILABLE KUBERNETES FURY DISTRIBUTION VERSIONS", lines[0])
	assert.Equal(t, "-----------------------------------------------", lines[1])
	assert.Equal(t, "VERSION\tRELEASE DATE\tEKS\tKFD\tON PREMISE", lines[2])
	assert.Contains(t, lines[3], "v1.31.0\t2024-10-06")
	assert.Contains(t, lines[4], "v1.30.0\t2023-10-06")
	assert.Contains(t, lines[5], "v1.29.0\t2022-10-06")
}
