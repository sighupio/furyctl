// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/cmd/create"
)

var (
	createCmd = &cobra.Command{ //nolint:gochecknoglobals // needed for cobra/viper compatibility.
		Use:   "create",
		Short: "Create a cluster or a sample configuration file",
	}
	clusterCmd = &cobra.Command{ //nolint:gochecknoglobals // needed for cobra/viper compatibility.
		Use:    "cluster",
		Short:  "Apply the configuration to create or upgrade a battle-tested Kubernetes Fury cluster",
		PreRun: applyCmd.PreRun,
		RunE:   applyCmd.RunE,
	}
)

//nolint:gochecknoinits // this pattern requires init function to work.
func init() {
	clusterCmd.Flags().AddFlagSet(applyCmd.Flags())

	createCmd.AddCommand(clusterCmd)
	createCmd.AddCommand(create.ConfigCmd)
}
