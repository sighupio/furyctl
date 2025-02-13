// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mocks

import (
	"github.com/sighupio/furyctl/internal/git"
)

// MockGitHubClient is a mocked version of GitHubClient.
type MockGitHubClient struct {
	releasesResponse []git.Release
}

// GetReleases mocks the GetReleases method of GitHubClient.
func (m MockGitHubClient) GetReleases() ([]git.Release, error) {
	return m.releasesResponse, nil
}

// NewMockGitHubClient creates a new MockGitHubClient with predefined responses.
func NewMockGitHubClient(releases []git.Release) git.RepoClient {
	return MockGitHubClient{
		releasesResponse: releases,
	}
}
