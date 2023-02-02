// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package netx_test

import (
	"net"
	"testing"

	netx "github.com/sighupio/furyctl/internal/x/net"
)

func TestAddOffsetToIPNet(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		ipNet  *net.IPNet
		offset int
		want   *net.IPNet
	}{
		{
			name:   "test valid ipnet",
			ipNet:  &net.IPNet{IP: net.IPv4(192, 168, 0, 0), Mask: net.CIDRMask(24, 32)},
			offset: 1,
			want:   &net.IPNet{IP: net.IPv4(192, 168, 0, 1), Mask: net.CIDRMask(24, 32)},
		},
		{
			name:   "test nil ipnet",
			ipNet:  nil,
			offset: 1,
			want:   nil,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := netx.AddOffsetToIPNet(tc.ipNet, tc.offset)

			if got.String() != tc.want.String() {
				t.Errorf("expected %v, got %v", tc.want, got)
			}
		})
	}
}
