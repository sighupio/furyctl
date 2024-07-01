// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"github.com/spf13/cobra"
)

var ConnectCmd = &cobra.Command{ //nolint:gochecknoglobals // needed for cobra/viper compatibility.
	Use:   "connect",
	Short: "Start up a new private connection to a cluster",
}

//nolint:gochecknoinits // this pattern requires init function to work.
func init() {
	RootCmd.AddCommand(ConnectCmd)
}
