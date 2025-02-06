// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package git

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

// HTTPClient is an interface for making HTTP requests.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// ClientConfig holds the configuration for the GitHub API client.
type ClientConfig struct {
	TagsAPI string
	Timeout time.Duration
}

type RepoClient interface {
	GetTags() ([]Tag, error)
	GetObjectInfo(sha string) (ObjectInfo, error)
}

// GitHubClient provides methods for interacting with the GitHub API.
//
//revive:disable-next-line:exported
type GitHubClient struct {
	client HTTPClient
	config ClientConfig
}

// Tag represents the Git tag structure from the GitHub API.
type Tag struct {
	Ref    string `json:"ref"`
	Type   string `json:"type"`
	Object TagRef `json:"object"`
}

// TagCommit represents the commit object within a tag.
type TagRef struct {
	SHA string `json:"sha"`
	URL string `json:"url"`
}

// Commit represents the commit details retrieved from GitHub.
type ObjectInfo struct {
	Author *Author `json:"author"`
	Tagger *Author `json:"tagger"`
}

// CommitAuthor holds the commit author’s details.
type Author struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Date  string `json:"date"`
}

var ErrGHRateLimit = errors.New("rate limited from github public api, retry in 1 hour")

// GetTags retrieves Git tags from the GitHub API.
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

	if resp.StatusCode == http.StatusTooManyRequests {
		return tags, ErrGHRateLimit
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return tags, fmt.Errorf("error reading from github api: %w", err)
	}

	if err := json.Unmarshal(respBody, &tags); err != nil {
		return tags, fmt.Errorf("error decoding response: %w", err)
	}

	return tags, nil
}

// GetObjectInfo fetches commit/tag details for a given URL.
func (gc GitHubClient) GetObjectInfo(url string) (ObjectInfo, error) {
	var objectInfo ObjectInfo

	ctx, cancel := context.WithTimeout(context.Background(), gc.config.Timeout)

	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return objectInfo, fmt.Errorf("error creating request: %w", err)
	}

	resp, err := gc.client.Do(req)
	if err != nil {
		return objectInfo, fmt.Errorf("error performing request: %w", err)
	}

	defer func() {
		if resp != nil && resp.Body != nil {
			if err := resp.Body.Close(); err != nil {
				logrus.Error(err)
			}
		}
	}()

	if resp.StatusCode == http.StatusTooManyRequests {
		return objectInfo, ErrGHRateLimit
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return objectInfo, fmt.Errorf("error reading from github api: %w", err)
	}

	if err := json.Unmarshal(respBody, &objectInfo); err != nil {
		return objectInfo, fmt.Errorf("error decoding response: %w", err)
	}

	return objectInfo, nil
}

const gitHubClientTimeout = 5 * time.Second

// NewGitHubClient creates a new GitHub client with the given configuration.
func NewGitHubClient() *GitHubClient {
	return &GitHubClient{
		client: http.DefaultClient,
		config: ClientConfig{
			TagsAPI: "https://api.github.com/repos/sighupio/fury-distribution/git/refs/tags",
			Timeout: gitHubClientTimeout,
		},
	}
}
