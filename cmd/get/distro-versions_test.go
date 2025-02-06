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
			"https://.../31": {Tagger: &git.Author{Name: "John Doe", Email: "john@example.com", Date: "2024-10-06T14:16:00Z"}},
			"https://.../30": {Author: &git.Author{Name: "John Doe", Email: "john@example.com", Date: "2023-10-06T14:16:00Z"}},
			"https://.../29": {Author: &git.Author{Name: "John Doe", Email: "john@example.com", Date: "2022-10-06T14:16:00Z"}},
			"https://.../28": {Tagger: &git.Author{Name: "John Doe", Email: "john@example.com", Date: "2020-10-06T14:16:00Z"}},
			"https://.../27": {Tagger: &git.Author{Name: "John Doe", Email: "john@example.com", Date: "2019-10-06T14:16:00Z"}},
			"https://.../23": {Author: &git.Author{Name: "John Doe", Email: "john@example.com", Date: "2018-10-06T14:16:00Z"}},
			"https://.../22": {Author: &git.Author{Name: "John Doe", Email: "john@example.com", Date: "2023-10-06T14:16:00Z"}},
			"https://.../20": {Author: &git.Author{Name: "John Doe", Email: "john@example.com", Date: "2023-10-06T14:16:00Z"}},
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
