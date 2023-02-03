// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package netx_test

import (
	"errors"
	"net"
	"testing"

	netx "github.com/sighupio/furyctl/internal/x/net"
)

func TestAddOffsetToIPNet(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		ipNet   *net.IPNet
		offset  int
		want    *net.IPNet
		wantErr *error
	}{
		{
			name:    "test valid ipnet positive offset",
			ipNet:   &net.IPNet{IP: net.IPv4(192, 168, 0, 0), Mask: net.CIDRMask(24, 32)},
			offset:  1,
			want:    &net.IPNet{IP: net.IPv4(192, 168, 0, 1), Mask: net.CIDRMask(24, 32)},
			wantErr: nil,
		},
		{
			name:    "test valid ipnet negative offset",
			ipNet:   &net.IPNet{IP: net.IPv4(192, 168, 0, 1), Mask: net.CIDRMask(24, 32)},
			offset:  -1,
			want:    &net.IPNet{IP: net.IPv4(192, 168, 0, 0), Mask: net.CIDRMask(24, 32)},
			wantErr: nil,
		},
		{
			name:    "test nil ipnet",
			ipNet:   nil,
			offset:  1,
			want:    nil,
			wantErr: &netx.ErrIPNetIsNil,
		},
		{
			name:    "test invalid ipnet",
			ipNet:   &net.IPNet{IP: []byte{0, 168, 0, 0, 1}, Mask: net.CIDRMask(24, 32)},
			offset:  1,
			want:    nil,
			wantErr: &netx.ErrInvalidIP,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := netx.AddOffsetToIPNet(tc.ipNet, tc.offset)

			if tc.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", *tc.wantErr)
				}

				if !errors.Is(err, *tc.wantErr) {
					t.Errorf("expected error %v, got %v", *tc.wantErr, err)
				}

				return
			}

			if got.String() != tc.want.String() {
				t.Errorf("expected %v, got %v", tc.want, got)
			}
		})
	}
}
