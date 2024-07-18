// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package git

import (
	"errors"
	"fmt"
)

var ErrUnsupportedGitProtocol = errors.New("unsupported git protocol")

func NewProtocol(protocol string) (Protocol, error) {
	switch protocol {
	case "ssh":
		return ProtocolSSH, nil

	case "https":
		return ProtocolHTTPS, nil
	}

	return "", fmt.Errorf("%w: %s", ErrUnsupportedGitProtocol, protocol)
}

type Protocol string

const (
	ProtocolSSH   = Protocol("ssh")
	ProtocolHTTPS = Protocol("https")
)
