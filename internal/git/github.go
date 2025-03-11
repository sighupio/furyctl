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

const (
	GitHubDefaultPage   = 1
	GitHubPerPageLimit  = 100
	gitHubClientTimeout = 5 * time.Second
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

// GetReleases fetches all releases from GitHub.
func (gc GitHubClient) GetReleases() ([]Release, error) {
	var releases []Release

	page := GitHubDefaultPage
	perPage := GitHubPerPageLimit
	hasMorePages := true

	for hasMorePages {
		pageReleases, nextPage, err := gc.fetchReleasesPage(page, perPage)
		if err != nil {
			return releases, err
		}

		releases = append(releases, pageReleases...)

		if len(pageReleases) == 0 {
			hasMorePages = false
		} else {
			page = nextPage
		}
	}

	return releases, nil
}

func (gc GitHubClient) fetchReleasesPage(page, perPage int) ([]Release, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gc.config.Timeout)
	defer cancel()

	url := fmt.Sprintf("%s?per_page=%d&page=%d", gc.config.ReleaseAPI, perPage, page)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, page, fmt.Errorf("error creating request: %w", err)
	}

	resp, err := gc.client.Do(req)
	if err != nil {
		return nil, page, fmt.Errorf("error performing request: %w", err)
	}

	defer func() {
		if resp != nil && resp.Body != nil {
			if err := resp.Body.Close(); err != nil {
				logrus.Error(err)
			}
		}
	}()

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, page, ErrGHRateLimit
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, page, fmt.Errorf("error reading from GitHub API: %w", err)
	}

	var releases []Release
	if err := json.Unmarshal(respBody, &releases); err != nil {
		return nil, page, fmt.Errorf("error decoding response: %w", err)
	}

	return releases, page + 1, nil
}

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
