// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"github.com/sighupio/furyctl/cmd/create"
	"github.com/spf13/cobra"
)

func NewCreateCommand(version string) *cobra.Command {
	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a cluster",
	}

	createCmd.AddCommand(create.NewClusterCmd(version))

	return createCmd
}
