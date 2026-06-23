// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/furyctl/internal/semver"
)

type Release struct {
	//nolint:tagliatelle // GitHub response's field has snake case.
	URL     string `json:"html_url"`
	Version string `json:"name"`
}

const (
	latestSource = "https://api.github.com/repos/sighupio/furyctl/releases/latest"
	timeout      = 5 * time.Second
)

var ErrCannotCheckNewRelease = errors.New("cannot check for new release")

// CheckNewRelease checks if there is a new release available.
func CheckNewRelease(bv string) (string, error) {
	if bv == "" || bv == "unknown" {
		return "", nil
	}

	rel, err := GetLatestRelease()
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrCannotCheckNewRelease, err)
	}

	if rel.Version == "" {
		return "", fmt.Errorf("%w: fetched release is empty", ErrCannotCheckNewRelease)
	}

	latestVer, err := semver.NewVersion(rel.Version)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrCannotCheckNewRelease, err)
	}

	buildVer, err := semver.NewVersion(bv)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrCannotCheckNewRelease, err)
	}

	if latestVer.GreaterThan(buildVer) {
		return rel.Version, nil
	}

	return "", nil
}

// GetLatestRelease fetches the latest release from the GitHub API.
func GetLatestRelease() (Release, error) {
	var release Release

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, latestSource, nil)
	if err != nil {
		return release, fmt.Errorf("error creating request: %w", err)
	}

	// Authenticate the request when a GitHub token is available to avoid the low
	// unauthenticated API rate limit (60 req/h per IP), which otherwise makes the
	// release lookup return an empty/error response (e.g. in CI).
	if token := githubToken(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
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

	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return release, fmt.Errorf("error decoding response: %w", err)
	}

	return release, nil
}

// githubToken returns a GitHub API token from the environment, if any.
func githubToken() string {
	for _, key := range []string{"GITHUB_TOKEN", "GITHUB_API_TOKEN"} {
		if v := os.Getenv(key); v != "" {
			return v
		}
	}

	return ""
}
