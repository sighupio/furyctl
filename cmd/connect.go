// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/cmd/connect"
	"github.com/sighupio/furyctl/internal/analytics"
)

func NewConnectCommand(tracker *analytics.Tracker) (*cobra.Command, error) {
	connectCmd := &cobra.Command{
		Use:   "connect",
		Short: "Start up a new private connection to a cluster",
	}

	openvpnCmd, err := connect.NewOpenVPNCmd(tracker)
	if err != nil {
		return nil, fmt.Errorf("error while creating openvpn command: %w", err)
	}

	connectCmd.AddCommand(openvpnCmd)

	return connectCmd, nil
}
