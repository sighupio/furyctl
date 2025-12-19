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

// ExtractIPFromCIDR extracts the IP address from CIDR notation.
// Example: "192.168.100.10/24" → "192.168.100.10".
func ExtractIPFromCIDR(cidr string) (string, error) {
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		return "", fmt.Errorf("invalid CIDR format '%s': %w", cidr, err)
	}

	return prefix.Addr().String(), nil
}

// CIDRsOverlap checks if two CIDR ranges overlap.
func CIDRsOverlap(cidr1, cidr2 string) (bool, error) {
	prefix1, err := netip.ParsePrefix(cidr1)
	if err != nil {
		return false, fmt.Errorf("invalid CIDR '%s': %w", cidr1, err)
	}

	prefix2, err := netip.ParsePrefix(cidr2)
	if err != nil {
		return false, fmt.Errorf("invalid CIDR '%s': %w", cidr2, err)
	}

	return prefix1.Overlaps(prefix2), nil
}

// IPInCIDR checks if an IP address falls within a CIDR range.
func IPInCIDR(ip, cidr string) (bool, error) {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return false, fmt.Errorf("invalid IP '%s': %w", ip, err)
	}

	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		return false, fmt.Errorf("invalid CIDR '%s': %w", cidr, err)
	}

	return prefix.Contains(addr), nil
}
