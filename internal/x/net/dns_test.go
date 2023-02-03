// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build integration

package netx_test

import (
	"testing"

	netx "github.com/sighupio/furyctl/internal/x/net"
)

func TestDNSQuery(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		server  string
		target  string
		wantErr bool
	}{
		{
			name:    "test valid dns query",
			server:  "8.8.8.8",
			target:  "google.com.",
			wantErr: false,
		},
		{
			name:    "test invalid dns query",
			server:  "8.8.8.8",
			target:  "google.com",
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := netx.DNSQuery(tc.server, tc.target)

			if tc.wantErr && err == nil {
				t.Errorf("expected error, got nil")
			}

			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
