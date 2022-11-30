// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package create

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/internal/analytics"
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
	ErrParsingFlag                = errors.New("error while parsing flag")
	ErrDownloadDependenciesFailed = errors.New("download dependencies failed")
)

func NewClusterCmd(version string, tracker *analytics.Tracker) *cobra.Command {
	var cmdEvent analytics.Event

	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "Creates a battle-tested Kubernetes cluster",
		PreRun: func(cmd *cobra.Command, args []string) {
			cmdEvent = analytics.NewCommandEvent(cobrax.GetFullname(cmd))
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Get flags.
			debug, ok := cobrax.Flag[bool](cmd, "debug").(bool)
			if !ok {
				err := fmt.Errorf("%w: debug", ErrParsingFlag)

				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return err
			}
			furyctlPath, ok := cobrax.Flag[string](cmd, "config").(string)
			if !ok {
				err := fmt.Errorf("%w: config", ErrParsingFlag)

				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return err
			}
			distroLocation, ok := cobrax.Flag[string](cmd, "distro-location").(string)
			if !ok {
				err := fmt.Errorf("%w: distro-location", ErrParsingFlag)

				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return err
			}
			phase, ok := cobrax.Flag[string](cmd, "phase").(string)
			if !ok {
				err := fmt.Errorf("%w: phase", ErrParsingFlag)

				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return err
			}
			binPath := cobrax.Flag[string](cmd, "bin-path").(string) //nolint:errcheck,forcetypeassert // optional flag
			vpnAutoConnect, ok := cobrax.Flag[bool](cmd, "vpn-auto-connect").(bool)
			if !ok {
				err := fmt.Errorf("%w: vpn-auto-connect", ErrParsingFlag)

				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return err
			}
			dryRun, ok := cobrax.Flag[bool](cmd, "dry-run").(bool)
			if !ok {
				err := fmt.Errorf("%w: dry-run", ErrParsingFlag)

				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return err
			}
			skipDownload, ok := cobrax.Flag[bool](cmd, "skip-download").(bool)
			if !ok {
				err := fmt.Errorf("%w: skip-download", ErrParsingFlag)

				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return err
			}

			// Init paths.
			homeDir, err := os.UserHomeDir()
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while getting current working directory: %w", err)
			}

			if binPath == "" {
				binPath = filepath.Join(homeDir, ".furyctl", "bin")
			}

			// Init first half of collaborators.
			client := netx.NewGoGetterClient()
			executor := execx.NewStdExecutor()
			distrodl := distribution.NewDownloader(client)

			// Init packages.
			execx.Debug = debug

			// Download the distribution.
			logrus.Info("Downloading distribution...")
			res, err := distrodl.Download(version, distroLocation, furyctlPath)

			cmdEvent.AddClusterDetails(analytics.ClusterDetails{
				Provider:   res.DistroManifest.Kubernetes.Eks.Version,
				KFDVersion: res.DistroManifest.Version,
				Phase:      phase,
			})

			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while downloading distribution: %w", err)
			}

			basePath := filepath.Join(homeDir, ".furyctl", res.MinimalConf.Metadata.Name)

			// Init second half of collaborators.
			depsdl := dependencies.NewDownloader(client, basePath, binPath)
			depsvl := dependencies.NewValidator(executor, binPath)

			// Validate the furyctl.yaml file.
			logrus.Info("Validating furyctl.yaml file...")
			if err := config.Validate(furyctlPath, res.RepoPath); err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while validating furyctl.yaml file: %w", err)
			}

			// Download the dependencies.
			if !skipDownload {
				logrus.Info("Downloading dependencies...")
				if errs, _ := depsdl.DownloadAll(res.DistroManifest); len(errs) > 0 {
					cmdEvent.AddErrorMessage(ErrDownloadDependenciesFailed)
					tracker.Track(cmdEvent)

					return fmt.Errorf("%w: %v", ErrDownloadDependenciesFailed, errs)
				}
			}

			// Validate the dependencies.
			logrus.Info("Validating dependencies...")
			if err := depsvl.Validate(res); err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while validating dependencies: %w", err)
			}

			// Create the cluster.
			clusterCreator, err := cluster.NewCreator(
				res.MinimalConf,
				res.DistroManifest,
				basePath,
				res.RepoPath,
				binPath,
				furyctlPath,
				phase,
				vpnAutoConnect,
			)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while initializing cluster creation: %w", err)
			}

			logrus.Info("Creating cluster...")
			if err := clusterCreator.Create(dryRun); err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while creating cluster: %w", err)
			}

			if !dryRun && phase == "" {
				logrus.Info("Cluster created successfully!")
			}

			if phase != "" {
				logrus.Infof("Phase %s executed successfully!", phase)
			}

			if _, err := fmt.Println("cluster creation succeeded"); err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while printing success message: %w", err)
			}

			cmdEvent.AddSuccessMessage("cluster creation succeeded")
			tracker.Track(cmdEvent)

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

	cmd.Flags().StringP(
		"bin-path",
		"b",
		"",
		"Path to the bin folder where all dependencies are installed",
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
