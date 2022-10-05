// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app_test

import (
	"testing"

	"github.com/sighupio/furyctl/internal/app"
)

func Test_Update_FetchLastRelease(t *testing.T) {
	tests := []struct {
		name string
		want app.Release
	}{
		{
			name: "test",
			want: app.Release{
				URL:     "https://github.com/sighupio/furyctl/releases/tag/v0.8.0",
				Version: "v0.8.0",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := app.GetLatestRelease()
			if err != nil {
				t.Fatal(err)
			}

			t.Log(got)

			if got.Version != tt.want.Version {
				t.Errorf("FetchLastRelease() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_Update_MustUpdate(t *testing.T) {
	tests := []struct {
		name           string
		want           bool
		currentVersion string
		latestVersion  string
	}{
		{
			name:           "Versions are equal, no update",
			currentVersion: "v0.8.0",
			latestVersion:  "v0.8.0",
			want:           false,
		},
		{
			name:           "Current version is higher than latest, no update",
			currentVersion: "v0.10.0",
			latestVersion:  "v0.8.0",
			want:           false,
		},
		{
			name:           "Current version's lenght is higher than latest, no update",
			currentVersion: "v0.0.0.1",
			latestVersion:  "v0.0.0",
			want:           false,
		},
		{
			name:           "Current version is lower than latest, update",
			currentVersion: "v0.7.0",
			latestVersion:  "v0.8.0",
			want:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := app.ShouldUpdate(tt.currentVersion, tt.latestVersion); got != tt.want {
				t.Errorf("%s = got %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}
