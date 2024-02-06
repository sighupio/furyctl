// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build integration

package app_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sighupio/furyctl/internal/app"
	"github.com/sighupio/furyctl/internal/test"
)

func TestGetLatestRelease(t *testing.T) {
	got, err := app.GetLatestRelease()
	if err != nil {
		t.Fatal(err)
	}

	if got.Version == "" {
		t.Error("Version is empty")
	}

	if got.URL == "" {
		t.Error("Version is empty")
	}
}

func TestCheckNewRelease(t *testing.T) {
	t.Parallel()

	got, err := app.GetLatestRelease()
	if err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		desc         string
		buildVersion string
		want         string
		wantErr      error
	}{
		{
			desc:         "empty build release",
			buildVersion: "",
			want:         "",
			wantErr:      nil,
		},
		{
			desc:         "unknown build release",
			buildVersion: "unknown",
			want:         "",
			wantErr:      nil,
		},
		{
			desc:         "old release",
			buildVersion: "v0.27.0",
			want:         got.Version,
			wantErr:      nil,
		},
		{
			desc:         "same release",
			buildVersion: got.Version,
			want:         "",
			wantErr:      nil,
		},
	}
	for _, tC := range testCases {
		tC := tC

		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			got, err := app.CheckNewRelease(tC.buildVersion)

			assert.Equal(t, tC.want, got)
			test.AssertErrorIs(t, err, tC.wantErr)
		})
	}
}
