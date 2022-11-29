// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"github.com/spf13/cobra"

	del "github.com/sighupio/furyctl/cmd/delete"
)

func NewDeleteCommand(version string) *cobra.Command {
	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a cluster",
	}

	deleteCmd.AddCommand(del.NewClusterCmd(version))

	return deleteCmd
}
