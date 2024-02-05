package git

import "fmt"

var ErrUnsupportedGitProtocol = fmt.Errorf("unsupported git protocol")

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
