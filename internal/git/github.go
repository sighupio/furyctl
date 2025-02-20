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
	ReleaseAPI string
	Timeout    time.Duration
}

type RepoClient interface {
	GetReleases() ([]Release, error)
}

type Release struct {
	//nolint:tagliatelle // GitHub response's field has snake case.
	TagName string `json:"tag_name"`
	//nolint:tagliatelle // GitHub response's field has snake case.
	PublishedAt time.Time `json:"published_at"`
	//nolint:tagliatelle // GitHub response's field has snake case.
	CreatedAt  time.Time `json:"created_at"`
	PreRelease bool      `json:"prerelease"`
}

// GitHubClient provides methods for interacting with the GitHub API.
//
//revive:disable-next-line:exported
type GitHubClient struct {
	client HTTPClient
	config ClientConfig
}

var ErrGHRateLimit = errors.New("rate limited from GitHub public API, retry in 1 hour")

func (gc GitHubClient) GetReleases() ([]Release, error) {
	var release []Release

	ctx, cancel := context.WithTimeout(context.Background(), gc.config.Timeout)

	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, gc.config.ReleaseAPI, nil)
	if err != nil {
		return release, fmt.Errorf("error creating request: %w", err)
	}

	resp, err := gc.client.Do(req)
	if err != nil {
		return release, fmt.Errorf("error performing request: %w", err)
	}

	defer func() {
		if resp != nil && resp.Body != nil {
			if err := resp.Body.Close(); err != nil {
				logrus.Error(err)
			}
		}
	}()

	if resp.StatusCode == http.StatusTooManyRequests {
		return release, ErrGHRateLimit
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return release, fmt.Errorf("error reading from GitHub API: %w", err)
	}

	if err := json.Unmarshal(respBody, &release); err != nil {
		return release, fmt.Errorf("error decoding response: %w", err)
	}

	return release, nil
}

const gitHubClientTimeout = 5 * time.Second

// NewGitHubClient creates a new GitHub client with the given configuration.
func NewGitHubClient() *GitHubClient {
	return &GitHubClient{
		client: http.DefaultClient,
		config: ClientConfig{
			ReleaseAPI: "https://api.github.com/repos/sighupio/fury-distribution/releases",
			Timeout:    gitHubClientTimeout,
		},
	}
}
