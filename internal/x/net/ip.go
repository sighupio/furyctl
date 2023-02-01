package netx

import (
	"encoding/binary"
	"net"
)

func AddOffsetToIpNet(ipNet *net.IPNet, offset int) *net.IPNet {
	if ipNet == nil {
		return nil
	}

	var networkAddress uint32

	if len(ipNet.IP) == net.IPv6len {
		networkAddress = binary.BigEndian.Uint32(ipNet.IP[(net.IPv6len - net.IPv4len):net.IPv6len])
	} else {
		networkAddress = binary.BigEndian.Uint32(ipNet.IP)
	}

	networkAddress += uint32(offset)

	newIP := make(net.IP, net.IPv4len)

	binary.BigEndian.PutUint32(newIP, networkAddress)

	return &net.IPNet{IP: newIP, Mask: ipNet.Mask}
}
