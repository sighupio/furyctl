// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package create

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	// Running init to register the EKSCluster kind.
	_ "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/eks"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/config"
	"github.com/sighupio/furyctl/internal/dependencies"
	"github.com/sighupio/furyctl/internal/distribution"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	netx "github.com/sighupio/furyctl/internal/x/net"
)

var (
	ErrDebugFlagNotSet          = errors.New("debug flag not set")
	ErrFuryctlFlagNotSet        = errors.New("furyctl flag not set")
	ErrDistroFlagNotSet         = errors.New("distro flag not set")
	ErrPhaseFlagNotSet          = errors.New("phase flag not set")
	ErrVpnAutoConnectFlagNotSet = errors.New("vpn-auto-connect flag not set")
	ErrDryRunFlagNotSet         = errors.New("dry-run flag not set")
	ErrSkipDownloadFlagNotSet   = errors.New("skip-download flag not set")

	ErrDownloadDependenciesFailed = errors.New("download dependencies failed")
)

func NewClusterCmd(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "Creates a battle-tested Kubernetes cluster",
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Get flags.
			debug, ok := cobrax.Flag[bool](cmd, "debug").(bool)
			if !ok {
				return ErrDebugFlagNotSet
			}
			furyctlPath, ok := cobrax.Flag[string](cmd, "config").(string)
			if !ok {
				return ErrFuryctlFlagNotSet
			}
			distroLocation, ok := cobrax.Flag[string](cmd, "distro-location").(string)
			if !ok {
				return ErrDistroFlagNotSet
			}
			phase, ok := cobrax.Flag[string](cmd, "phase").(string)
			if !ok {
				return ErrPhaseFlagNotSet
			}
			vpnAutoConnect, ok := cobrax.Flag[bool](cmd, "vpn-auto-connect").(bool)
			if !ok {
				return ErrVpnAutoConnectFlagNotSet
			}
			dryRun, ok := cobrax.Flag[bool](cmd, "dry-run").(bool)
			if !ok {
				return ErrDryRunFlagNotSet
			}
			skipDownload, ok := cobrax.Flag[bool](cmd, "skip-download").(bool)
			if !ok {
				return ErrSkipDownloadFlagNotSet
			}

			// Init paths.
			basePath, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("error while getting current working directory: %w", err)
			}

			binPath := filepath.Join(basePath, "vendor", "bin")

			// Init collaborators.
			client := netx.NewGoGetterClient()
			executor := execx.NewStdExecutor()
			distrodl := distribution.NewDownloader(client, debug)
			depsdl := dependencies.NewDownloader(client, basePath)
			depsvl := dependencies.NewValidator(executor, binPath)

			// Init packages.
			execx.Debug = debug

			// Download the distribution.
			res, err := distrodl.Download(version, distroLocation, furyctlPath)
			if err != nil {
				return fmt.Errorf("error while downloading distribution: %w", err)
			}

			// Validate the furyctl.yaml file.
			if err := config.Validate(furyctlPath, res.RepoPath); err != nil {
				return fmt.Errorf("error while validating furyctl.yaml file: %w", err)
			}

			// Download the dependencies.
			if !skipDownload {
				if errs, _ := depsdl.DownloadAll(res.DistroManifest); len(errs) > 0 {
					return fmt.Errorf("%w: %v", ErrDownloadDependenciesFailed, errs)
				}
			}

			// Validate the dependencies.
			if err := depsvl.Validate(res); err != nil {
				return fmt.Errorf("error while validating dependencies: %w", err)
			}

			// Create the cluster.
			clusterCreator, err := cluster.NewCreator(
				res.MinimalConf,
				res.DistroManifest,
				res.RepoPath,
				furyctlPath,
				phase,
				vpnAutoConnect,
			)
			if err != nil {
				return fmt.Errorf("error while initializing cluster creation: %w", err)
			}

			if err := clusterCreator.Create(dryRun); err != nil {
				return fmt.Errorf("error while creating cluster: %w", err)
			}

			_, err = fmt.Println("cluster creation succeeded")
			if err != nil {
				return fmt.Errorf("error while printing success message: %w", err)
			}

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
