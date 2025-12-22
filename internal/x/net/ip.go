// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package netx

import (
	"errors"
	"fmt"
	"math"
	"net"
	"net/netip"
)

var (
	ErrInvalidIP  = errors.New("invalid ip")
	ErrIPNetIsNil = errors.New("ipnet is nil")
)

func AddOffsetToIPNet(ipNet *net.IPNet, offset int) (*net.IPNet, error) {
	if ipNet == nil {
		return nil, ErrIPNetIsNil
	}

	newIP, err := netip.ParseAddr(ipNet.IP.String())
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidIP, err)
	}

	for range int(math.Abs(float64(offset))) {
		if offset > 0 {
			newIP = newIP.Next()

			continue
		}

		newIP = newIP.Prev()
	}

	return &net.IPNet{IP: newIP.AsSlice(), Mask: ipNet.Mask}, nil
}
