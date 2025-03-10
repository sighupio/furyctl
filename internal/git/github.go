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
	var releases []Release
	page := 1
	perPage := 100
	hasMorePages := true

	for hasMorePages {
		ctx, cancel := context.WithTimeout(context.Background(), gc.config.Timeout)
		defer cancel()

		url := fmt.Sprintf("%s?per_page=%d&page=%d", gc.config.ReleaseAPI, perPage, page)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return releases, fmt.Errorf("error creating request: %w", err)
		}

		resp, err := gc.client.Do(req)
		if err != nil {
			return releases, fmt.Errorf("error performing request: %w", err)
		}

		defer func() {
			if resp != nil && resp.Body != nil {
				if err := resp.Body.Close(); err != nil {
					logrus.Error(err)
				}
			}
		}()

		if resp.StatusCode == http.StatusTooManyRequests {
			return releases, ErrGHRateLimit
		}

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return releases, fmt.Errorf("error reading from GitHub API: %w", err)
		}

		var currentReleases []Release
		if err := json.Unmarshal(respBody, &currentReleases); err != nil {
			return releases, fmt.Errorf("error decoding response: %w", err)
		}

		if len(currentReleases) == 0 {
			hasMorePages = false
		} else {
			releases = append(releases, currentReleases...)
			page++
		}
	}

	return releases, nil
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
