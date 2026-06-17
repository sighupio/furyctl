// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

//nolint:testpackage // withChecksum is unexported and has no network-free public entry point.
package dependencies

import (
	"fmt"
	"runtime"
	"testing"
)

func Test_withChecksum(t *testing.T) {
	t.Parallel()

	const src = "https://example.com/ansible-portable-v0.2.1-linux-amd64.tar.gz"

	platformKey := runtime.GOOS + "-" + runtime.GOARCH
	hash := "deadbeef"

	testCases := []struct {
		desc      string
		checksums map[string]string
		want      string
	}{
		{
			desc:      "nil checksums returns src unchanged",
			checksums: nil,
			want:      src,
		},
		{
			desc:      "empty checksums returns src unchanged",
			checksums: map[string]string{},
			want:      src,
		},
		{
			desc:      "checksum for another platform returns src unchanged",
			checksums: map[string]string{"other-os-other-arch": "abc123"},
			want:      src,
		},
		{
			desc:      "empty hash for current platform returns src unchanged",
			checksums: map[string]string{platformKey: ""},
			want:      src,
		},
		{
			desc:      "checksum for current platform is appended",
			checksums: map[string]string{platformKey: hash},
			want:      fmt.Sprintf("%s?checksum=sha256:%s", src, hash),
		},
	}

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			got := withChecksum(src, tC.checksums)
			if got != tC.want {
				t.Errorf("withChecksum() = %q, want %q", got, tC.want)
			}
		})
	}
}
