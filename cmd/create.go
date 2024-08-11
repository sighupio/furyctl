// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/cmd/create"
)

func NewClusterCmd() *cobra.Command {
	applyCmd := NewApplyCmd()

	clusterCmd := &cobra.Command{
		Use:    "cluster",
		Short:  "Apply the configuration to create or upgrade a battle-tested Kubernetes Fury cluster",
		PreRun: applyCmd.PreRun,
		RunE:   applyCmd.RunE,
	}

	clusterCmd.Flags().AddFlagSet(applyCmd.Flags())

	return clusterCmd
}

func NewCreateCmd() *cobra.Command {
	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a cluster or a sample configuration file",
	}

	createCmd.AddCommand(NewClusterCmd())
	createCmd.AddCommand(create.NewConfigCmd())
	createCmd.AddCommand(create.NewPKICmd())

	return createCmd
}
