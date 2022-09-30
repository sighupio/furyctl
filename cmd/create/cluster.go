// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package create

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	_ "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/eks"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/cobrax"
	"github.com/sighupio/furyctl/internal/config"
	"github.com/sighupio/furyctl/internal/dependencies"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/execx"
	"github.com/sighupio/furyctl/internal/netx"
)

func NewClusterCmd(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "Creates a battle-tested Kubernetes cluster",
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Get flags
			debug := cobrax.Flag[bool](cmd, "debug").(bool)
			furyctlPath := cobrax.Flag[string](cmd, "config").(string)
			distroLocation := cobrax.Flag[string](cmd, "distro-location").(string)
			phase := cobrax.Flag[string](cmd, "phase").(string)
			vpnAutoConnect := cobrax.Flag[bool](cmd, "vpn-auto-connect").(bool)
			dryRun := cobrax.Flag[bool](cmd, "dry-run").(bool)
			skipDownload := cobrax.Flag[bool](cmd, "skip-download").(bool)

			// Init paths
			basePath, err := os.Getwd()
			if err != nil {
				return err
			}

			vendorPath := filepath.Join(basePath, "vendor")

			// Init collaborators
			client := netx.NewGoGetterClient()
			executor := execx.NewStdExecutor()
			distrodl := distribution.NewDownloader(client, debug)
			depsdl := dependencies.NewDownloader(client, basePath)
			depsvl := dependencies.NewValidator(executor)

			// Download the distribution
			res, err := distrodl.Download(version, distroLocation, furyctlPath)
			if err != nil {
				return err
			}

			// Validate the furyctl.yaml file
			if err := config.Validate(furyctlPath, res.RepoPath); err != nil {
				return err
			}

			// Download the dependencies
			if !skipDownload {
				if errs, _ := depsdl.DownloadAll(res.DistroManifest); len(errs) > 0 {
					return fmt.Errorf("errors downloading dependencies: %v", errs)
				}
			}

			// Validate the dependencies
			if err := depsvl.Validate(res, vendorPath); err != nil {
				return err
			}

			// Create the cluster
			clusterCreator, err := cluster.NewCreator(
				res.MinimalConf,
				res.DistroManifest,
				furyctlPath,
				phase,
				vpnAutoConnect,
			)
			if err != nil {
				return err
			}

			if err := clusterCreator.Create(dryRun); err != nil {
				return err
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
		"skip-download",
		false,
		"Skip downloading the distribution modules, installers and binaries",
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
