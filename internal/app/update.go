// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

type Release struct {
	//nolint:tagliatelle // GitHub response's field has snake case.
	URL     string `json:"html_url"`
	Version string `json:"name"`
}

const (
	latestSource = "https://api.github.com/repos/sighupio/furyctl/releases/latest"
	timeout      = 30 * time.Second
)

// GetLatestRelease fetches the latest release from the GitHub API.
func GetLatestRelease() (Release, error) {
	var release Release

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, latestSource, nil)
	if err != nil {
		return release, fmt.Errorf("error creating request: %w", err)
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
