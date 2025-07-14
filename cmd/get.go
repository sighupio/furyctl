// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/cmd/get"
)

func NewGetCmd() *cobra.Command {
	getCmd := &cobra.Command{
		Use:   "get",
		Short: "Get the kubeconfig, available upgrade paths for a cluster, or compatible versions",
	}

	getCmd.AddCommand(get.NewKubeconfigCmd())
	getCmd.AddCommand(get.NewUpgradePathsCmd())
	getCmd.AddCommand(get.NewSupportedVersionsCmd())

	return getCmd
}
