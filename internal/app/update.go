// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Release struct {
	URL     string `json:"html_url"`
	Version string `json:"name"`
}

// GetLatestRelease fetches the latest release from the GitHub API
func GetLatestRelease() (Release, error) {
	var release Release

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/sighupio/furyctl/releases/latest", nil)
	if err != nil {
		return release, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return release, err
	}

	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return release, err
	}

	return release, nil
}

// ShouldUpdate checks if the current version is outdated
func ShouldUpdate(currentVersion, latestVersion string) bool {
	// normalize the versions
	nc := strings.TrimPrefix(currentVersion, "v")
	nl := strings.TrimPrefix(latestVersion, "v")

	return compareVersions(nc, nl)
}

// util func to compare two semantic versions, e.g: 0.8.0 vs 0.9.0
func compareVersions(currentVersion, latestVersion string) bool {
	if currentVersion == latestVersion {
		return false
	}

	// split the versions into slices of strings
	currentVersionSlice := strings.Split(currentVersion, ".")
	latestVersionSlice := strings.Split(latestVersion, ".")

	if len(currentVersionSlice) < len(latestVersionSlice) {
		return true
	}

	if len(currentVersionSlice) > len(latestVersionSlice) {
		return false
	}

	// compare the versions
	for i := 0; i < len(currentVersionSlice); i++ {
		c, _ := strconv.Atoi(currentVersionSlice[i])
		l, _ := strconv.Atoi(latestVersionSlice[i])

		// if the current version is greater than the latest version
		if c > l {
			return false
		}
		// if the current version is less than the latest version
		if c < l {
			return true
		}
	}

	return false
}
