// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/cmd/create"
	"github.com/sighupio/furyctl/internal/analytics"
)

func NewCreateCommand(tracker *analytics.Tracker) *cobra.Command {
	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a cluster or a sample configuration file",
	}

	createCmd.AddCommand(create.NewClusterCmd(tracker))
	createCmd.AddCommand(create.NewConfigCmd(tracker))

	return createCmd
}
