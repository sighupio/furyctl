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
