// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

import (
	"context"
	"encoding/json"
	"net/http"
)

type Update struct {
	FuryctlBinVersion string
}

type Release struct {
	URL     string `json:"html_url"`
	Version string `json:"name"`
}

func NewUpdate(furyctlBinVersion string) *Update {
	return &Update{
		FuryctlBinVersion: furyctlBinVersion,
	}
}

func (u *Update) FetchLastRelease() (Release, error) {
	var release Release

	ctx := context.Background()
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
