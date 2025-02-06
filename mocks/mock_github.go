// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mocks

import (
	"errors"

	"github.com/sighupio/furyctl/internal/git"
)

// MockGitHubClient is a mocked version of GitHubClient.
type MockGitHubClient struct {
	tagsResponse   []git.Tag
	commitResponse map[string]git.ObjectInfo
}

// GetTags mocks the GetTags method of GitHubClient.
func (m MockGitHubClient) GetTags() ([]git.Tag, error) {
	return m.tagsResponse, nil
}

var ErrGitHubMock = errors.New("commit not found")

// GetCommit mocks the GetCommit method of GitHubClient.
func (m MockGitHubClient) GetObjectInfo(url string) (git.ObjectInfo, error) {
	if commit, ok := m.commitResponse[url]; ok {
		return commit, nil
	}

	return git.ObjectInfo{}, ErrGitHubMock
}

// NewMockGitHubClient creates a new MockGitHubClient with predefined responses.
func NewMockGitHubClient(tags []git.Tag, commits map[string]git.ObjectInfo) git.RepoClient {
	return MockGitHubClient{
		tagsResponse:   tags,
		commitResponse: commits,
	}
}
