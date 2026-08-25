// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package git_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/sighupio/furyctl/internal/git"
	"github.com/sighupio/furyctl/internal/test"
)

// TestParseProtocol covers the supported protocols and the rejection of anything
// else. Matching is exact: a protocol differing only by case is not accepted.
func TestParseProtocol(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc     string
		protocol string
		want     git.Protocol
		wantErr  error
	}{
		{
			desc:     "ssh protocol",
			protocol: "ssh",
			want:     git.ProtocolSSH,
		},
		{
			desc:     "https protocol",
			protocol: "https",
			want:     git.ProtocolHTTPS,
		},
		{
			desc:     "matching is case sensitive",
			protocol: "SSH",
			want:     "",
			wantErr:  git.ErrUnsupportedGitProtocol,
		},
		{
			desc:     "unsupported protocol",
			protocol: "example",
			want:     "",
			wantErr:  git.ErrUnsupportedGitProtocol,
		},
		{
			desc:     "empty protocol",
			protocol: "",
			want:     "",
			wantErr:  git.ErrUnsupportedGitProtocol,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			p, err := git.ParseProtocol(tC.protocol)

			if p != tC.want {
				t.Errorf("got: %s, want: %s", p, tC.want)
			}

			test.AssertErrorIs(t, err, tC.wantErr)
		})
	}
}

// TestParseProtocolErrorListsSupported ensures the rejection message tells the user
// which protocols are accepted.
func TestParseProtocolErrorListsSupported(t *testing.T) {
	t.Parallel()

	_, err := git.ParseProtocol("example")
	if err == nil {
		t.Fatal("got no error, want ErrUnsupportedGitProtocol")
	}

	for _, p := range git.ProtocolsS() {
		if !strings.Contains(err.Error(), p) {
			t.Errorf("error message %q does not mention supported protocol %q", err.Error(), p)
		}
	}
}

// TestProtocolsS checks the string view of Protocols stays aligned with it, in order.
func TestProtocolsS(t *testing.T) {
	t.Parallel()

	want := []string{"ssh", "https"}
	if got := git.ProtocolsS(); !slices.Equal(got, want) {
		t.Errorf("got: %v, want: %v", got, want)
	}
}
