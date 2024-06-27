// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package semver_test

import (
	"testing"

	"github.com/sighupio/furyctl/internal/semver"
)

func TestEnsurePrefix(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc    string
		version string
		want    string
	}{
		{
			desc:    "has prefix",
			version: "v1.2.3",
			want:    "v1.2.3",
		},
		{
			desc:    "has no prefix",
			version: "1.2.3",
			want:    "v1.2.3",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			got := semver.EnsurePrefix(tC.version)
			if tC.want != got {
				t.Errorf("want %q, got %q", tC.want, got)
			}
		})
	}
}

func TestEnsureNoPrefix(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc    string
		version string
		want    string
	}{
		{
			desc:    "has prefix",
			version: "v1.2.3",
			want:    "1.2.3",
		},
		{
			desc:    "has no prefix",
			version: "1.2.3",
			want:    "1.2.3",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			got := semver.EnsureNoPrefix(tC.version)
			if tC.want != got {
				t.Errorf("want %q, got %q", tC.want, got)
			}
		})
	}
}

func TestEnsureNoBuild(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc    string
		version string
		want    string
	}{
		{
			desc:    "has build using plus",
			version: "v1.2.3+b1",
			want:    "v1.2.3",
		},
		{
			desc:    "has build using dash",
			version: "v1.2.3-1",
			want:    "v1.2.3",
		},
		{
			desc:    "has no build",
			version: "1.2.3",
			want:    "1.2.3",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			got := semver.EnsureNoBuild(tC.version)
			if tC.want != got {
				t.Errorf("want %q, got %q", tC.want, got)
			}
		})
	}
}
