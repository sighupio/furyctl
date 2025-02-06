package git

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

// HTTPClient is an interface for making HTTP requests
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// ClientConfig holds the configuration for the GitHub API client
type ClientConfig struct {
	TagsAPI   string
	CommitAPI string
	Timeout   time.Duration
}

type RepoClient interface {
	GetTags() ([]Tag, error)
	GetCommit(sha string) (Commit, error)
}

// GitHubClient provides methods for interacting with the GitHub API
type GitHubClient struct {
	client HTTPClient
	config ClientConfig
}

// Tag represents the Git tag structure from the GitHub API.
type Tag struct {
	Ref    string    `json:"ref"`
	Object TagCommit `json:"object"`
}

// TagCommit represents the commit object within a tag.
type TagCommit struct {
	SHA string `json:"sha"`
	URL string `json:"url"`
}

// Commit represents the commit details retrieved from GitHub.
type Commit struct {
	Author CommitAuthor `json:"author"`
}

// CommitAuthor holds the commit author’s details.
type CommitAuthor struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Date  string `json:"date"`
}

type GithubMessage struct {
	Message string `json:"message"`
}

// GetTags retrieves Git tags from the GitHub API
func (gc GitHubClient) GetTags() ([]Tag, error) {
	var tags []Tag
	ctx, cancel := context.WithTimeout(context.Background(), gc.config.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, gc.config.TagsAPI, nil)
	if err != nil {
		return tags, fmt.Errorf("error creating request: %w", err)
	}

	resp, err := gc.client.Do(req)
	if err != nil {
		return tags, fmt.Errorf("error performing request: %w", err)
	}
	defer func() {
		if resp != nil && resp.Body != nil {
			if err := resp.Body.Close(); err != nil {
				logrus.Error(err)
			}
		}
	}()

	respBody, _ := io.ReadAll(resp.Body)

	if err := json.Unmarshal(respBody, &tags); err != nil {
		var ghMessage GithubMessage

		if err := json.Unmarshal(respBody, &ghMessage); err == nil {
			return tags, fmt.Errorf("error from github api: %s", ghMessage.Message)
		}
		return tags, fmt.Errorf("error decoding response: %w", err)
	}

	return tags, nil
}

// GetCommit fetches commit details for a given SHA
func (gc GitHubClient) GetCommit(sha string) (Commit, error) {
	var commit Commit
	ctx, cancel := context.WithTimeout(context.Background(), gc.config.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, gc.config.CommitAPI+sha, nil)
	if err != nil {
		return commit, fmt.Errorf("error creating request: %w", err)
	}

	resp, err := gc.client.Do(req)
	if err != nil {
		return commit, fmt.Errorf("error performing request: %w", err)
	}
	defer func() {
		if resp != nil && resp.Body != nil {
			if err := resp.Body.Close(); err != nil {
				logrus.Error(err)
			}
		}
	}()

	respBody, _ := io.ReadAll(resp.Body)

	if err := json.Unmarshal(respBody, &commit); err != nil {
		var ghMessage GithubMessage

		if err := json.Unmarshal(respBody, &ghMessage); err == nil {
			return commit, fmt.Errorf("error from github api: %s", ghMessage.Message)
		}
		return commit, fmt.Errorf("error decoding response: %w", err)
	}
	return commit, nil
}

// NewGitHubClient creates a new GitHub client with the given configuration
func NewGitHubClient() *GitHubClient {
	return &GitHubClient{
		client: http.DefaultClient,
		config: ClientConfig{
			TagsAPI:   "https://api.github.com/repos/sighupio/fury-distribution/git/refs/tags",
			CommitAPI: "https://api.github.com/repos/sighupio/fury-distribution/git/commits/",
			Timeout:   5 * time.Second,
		},
	}
}
