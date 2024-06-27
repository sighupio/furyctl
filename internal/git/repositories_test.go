// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package git_test

import (
	"testing"

	"github.com/sighupio/furyctl/internal/git"
	"github.com/sighupio/furyctl/internal/test"
)

func TestRepoPrefixByProtocol(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc     string
		protocol string
		want     string
		wantErr  error
	}{
		{
			desc:     "ssh protocol",
			protocol: "ssh",
			want:     "git@github.com:sighupio",
		},
		{
			desc:     "https protocol",
			protocol: "https",
			want:     "https://github.com/sighupio",
		},
		{
			desc:     "unsupported protocol",
			protocol: "example",
			want:     "",
			wantErr:  git.ErrUnsupportedGitProtocol,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			rp, err := git.RepoPrefixByProtocol(git.Protocol(tC.protocol))

			if rp != tC.want {
				t.Errorf("got: %s, want: %s", rp, tC.want)
			}

			test.AssertErrorIs(t, err, tC.wantErr)
		})
	}
}

func TestStripPrefix(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc string
		repo string
		want string
	}{
		{
			desc: "empty string",
			repo: "",
			want: "",
		},
		{
			desc: "fury distribution repo - https",
			repo: "https://github.com/sighupio/fury-distribution.git",
			want: "fury-distribution.git",
		},
		{
			desc: "fury distribution repo - ssh",
			repo: "git@github.com:sighupio/fury-distribution.git",
			want: "fury-distribution.git",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			got := git.StripPrefix(tC.repo)
			if got != tC.want {
				t.Errorf("got: %s, want: %s", got, tC.want)
			}
		})
	}
}

func TestStripSuffix(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc string
		repo string
		want string
	}{
		{
			desc: "empty string",
			repo: "",
			want: "",
		},
		{
			desc: "fury distribution repo - https",
			repo: "https://github.com/sighupio/fury-distribution.git",
			want: "https://github.com/sighupio/fury-distribution",
		},
		{
			desc: "fury distribution repo - ssh",
			repo: "git@github.com:sighupio/fury-distribution.git",
			want: "git@github.com:sighupio/fury-distribution",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			got := git.StripSuffix(tC.repo)
			if got != tC.want {
				t.Errorf("got: %s, want: %s", got, tC.want)
			}
		})
	}
}

func TestCleanupRepoURL(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc string
		repo string
		want string
	}{
		{
			desc: "empty string",
			repo: "",
			want: "",
		},
		{
			desc: "fury distribution repo - https",
			repo: "https://github.com/sighupio/fury-distribution.git",
			want: "fury-distribution",
		},
		{
			desc: "fury distribution repo - ssh",
			repo: "git@github.com:sighupio/fury-distribution.git",
			want: "fury-distribution",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			got := git.CleanupRepoURL(tC.repo)
			if got != tC.want {
				t.Errorf("got: %s, want: %s", got, tC.want)
			}
		})
	}
}
