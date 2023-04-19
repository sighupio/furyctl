// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"github.com/spf13/cobra"

	del "github.com/sighupio/furyctl/cmd/delete"
	"github.com/sighupio/furyctl/internal/analytics"
)

func NewDeleteCommand(tracker *analytics.Tracker) *cobra.Command {
	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a cluster and its related infrastructure",
	}

	deleteCmd.AddCommand(del.NewClusterCmd(tracker))

	return deleteCmd
}
