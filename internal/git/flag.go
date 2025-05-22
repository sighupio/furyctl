// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package git

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type ProtocolFlag struct {
	Protocol Protocol
}

func (p *ProtocolFlag) String() string {
	return string(p.Protocol)
}

func (p *ProtocolFlag) Set(value string) error {
	protocol, err := NewProtocol(value)
	if err != nil {
		return fmt.Errorf("%w: \"%s\". Supported protocols are %s",
			ErrUnsupportedGitProtocol,
			value,
			strings.Join(ProtocolsS(), ", "),
		)
	}

	p.Protocol = protocol

	return nil
}

func (p *ProtocolFlag) Type() string { //nolint:revive // I know p is not used
	return "git-protocol"
}

func ProtocolFlagCompletion(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return ProtocolsS(), cobra.ShellCompDirectiveDefault
}
