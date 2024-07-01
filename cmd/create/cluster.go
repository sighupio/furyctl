// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package create

import (
	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/cmd"
)

var clusterCmd = &cobra.Command{ //nolint:gochecknoglobals // needed for cobra/viper compatibility.
	Use:    "cluster",
	Short:  "Apply the configuration to create or upgrade a battle-tested Kubernetes Fury cluster",
	PreRun: cmd.ApplyCmd.PreRun,
	RunE:   cmd.ApplyCmd.RunE,
}

//nolint:gochecknoinits // this pattern requires init function to work.
func init() {
	clusterCmd.Flags().AddFlagSet(cmd.ApplyCmd.Flags())

	cmd.CreateCmd.AddCommand(clusterCmd)
}
