// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/cmd/connect"
	"github.com/sighupio/furyctl/internal/analytics"
)

func NewConnectCommand(tracker *analytics.Tracker) *cobra.Command {
	connectCmd := &cobra.Command{
		Use: "connect",
	}

	connectCmd.AddCommand(connect.NewOpenVPNCmd(tracker))

	return connectCmd
}
