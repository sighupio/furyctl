// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/cmd/diff"
	"github.com/sighupio/furyctl/internal/analytics"
)

func NewDiffCommand(tracker *analytics.Tracker) *cobra.Command {
	diffCmd := &cobra.Command{
		Use:   "diff",
		Short: "Shows the difference between desired and cluster state",
	}

	diffCmd.AddCommand(diff.NewClusterCommand(tracker))

	return diffCmd
}
