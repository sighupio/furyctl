// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package git

import (
	"errors"
	"fmt"
	"strings"
)

var ErrUnsupportedGitProtocol = errors.New("unsupported git protocol")

func NewProtocol(protocol string) (Protocol, error) {
	switch protocol {
	case "ssh":
		return ProtocolSSH, nil

	case "https":
		return ProtocolHTTPS, nil
	}

	return "", fmt.Errorf("%w: %s. Supported protocols are %s",
		ErrUnsupportedGitProtocol,
		protocol,
		strings.Join(ProtocolsS(), ", "),
	)
}

type Protocol string

const (
	ProtocolSSH   = Protocol("ssh")
	ProtocolHTTPS = Protocol("https")
)

// Protocols returns a slice of Protocols that are supported.
func Protocols() []Protocol {
	return []Protocol{
		ProtocolSSH,
		ProtocolHTTPS,
	}
}

// ProtocolsS returns a slice of Strings representation of the Protocols that are supported.
func ProtocolsS() []string {
	protocols := []string{}
	for _, p := range Protocols() {
		protocols = append(protocols, string(p))
	}

	return protocols
}
