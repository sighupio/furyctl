package mocks

import (
	"fmt"

	"github.com/sighupio/furyctl/internal/git"
)

// MockGitHubClient is a mocked version of GitHubClient
type MockGitHubClient struct {
	tagsResponse   []git.Tag
	commitResponse map[string]git.Commit
}

// GetTags mocks the GetTags method of GitHubClient
func (m MockGitHubClient) GetTags() ([]git.Tag, error) {
	return m.tagsResponse, nil
}

// GetCommit mocks the GetCommit method of GitHubClient
func (m MockGitHubClient) GetCommit(sha string) (git.Commit, error) {
	if commit, ok := m.commitResponse[sha]; ok {
		return commit, nil
	}
	return git.Commit{}, fmt.Errorf("commit not found")
}

// NewMockGitHubClient creates a new MockGitHubClient with predefined responses
func NewMockGitHubClient(tags []git.Tag, commits map[string]git.Commit) git.RepoClient {
	return MockGitHubClient{
		tagsResponse:   tags,
		commitResponse: commits,
	}
}
