// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package create

import (
	"fmt"

	"github.com/sighupio/furyctl/internal/app"
	"github.com/sighupio/furyctl/internal/cobrax"
	"github.com/spf13/cobra"
)

var ErrClusterCreationFailed = fmt.Errorf("cluster creation failed")

func NewClusterCmd(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "Creates a battle-tested Kubernetes cluster",
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			debug := cobrax.Flag[bool](cmd, "debug").(bool)
			furyctlPath := cobrax.Flag[string](cmd, "config").(string)
			distroLocation := cobrax.Flag[string](cmd, "distro-location").(string)
			phase := cobrax.Flag[string](cmd, "phase").(string)
			vpnAutoConnect := cobrax.Flag[bool](cmd, "vpn-auto-connect").(bool)

			cc := app.NewCreateCluster()

			res, err := cc.Execute(app.CreateClusterRequest{
				FuryctlConfPath:   furyctlPath,
				FuryctlBinVersion: version,
				DistroLocation:    distroLocation,
				Phase:             phase,
				VpnAutoConnect:    vpnAutoConnect,
				Debug:             debug,
			})
			if err != nil {
				return err
			}

			if res.HasErrors() {
				fmt.Println(res.Error)

				return ErrClusterCreationFailed
			}

			fmt.Println("cluster creation succeeded")

			return nil
		},
	}

	cmd.Flags().StringP(
		"config",
		"c",
		"furyctl.yaml",
		"Path to the furyctl.yaml file",
	)

	cmd.Flags().StringP(
		"phase",
		"p",
		"",
		"Phase to execute",
	)

	cmd.Flags().StringP(
		"distro-location",
		"",
		"",
		"Base URL used to download schemas, defaults and the distribution manifest. "+
			"It can either be a local path(eg: /path/to/fury/distribution) or "+
			"a remote URL(eg: https://git@github.com/sighupio/fury-distribution?ref=BRANCH_NAME)."+
			"Any format supported by hashicorp/go-getter can be used.",
	)

	cmd.Flags().Bool(
		"dry-run",
		false,
		"Allows to inspect what resources will be created before applying them",
	)

	cmd.Flags().Bool(
		"vpn-auto-connect",
		false,
		"Automatically connect to the VPN after the infrastructure phase",
	)

	return cmd
}
