// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"github.com/spf13/cobra"

	del "github.com/sighupio/furyctl/cmd/delete"
)

var deleteCmd = &cobra.Command{ //nolint:gochecknoglobals // needed for cobra/viper compatibility.
	Use:   "delete",
	Short: "Delete a cluster and its related infrastructure",
}

//nolint:gochecknoinits // this pattern requires init function to work.
func init() {
	deleteCmd.AddCommand(del.ClusterCmd)
}
