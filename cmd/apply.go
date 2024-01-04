// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

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
	"github.com/sighupio/furyctl/internal/cmd/cmdutil"
	"github.com/sighupio/furyctl/internal/config"
	"github.com/sighupio/furyctl/internal/dependencies"
	"github.com/sighupio/furyctl/internal/distribution"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	netx "github.com/sighupio/furyctl/internal/x/net"
)

const WrappedErrMessage = "%w: %s"

var ErrDownloadDependenciesFailed = errors.New("dependencies download failed")

type ClusterCmdFlags struct {
	Debug               bool
	FuryctlPath         string
	DistroLocation      string
	Phase               string
	StartFrom           string
	BinPath             string
	SkipVpn             bool
	VpnAutoConnect      bool
	DryRun              bool
	SkipDepsDownload    bool
	SkipDepsValidation  bool
	NoTTY               bool
	HTTPS               bool
	Force               bool
	Timeout             int
	Outdir              string
	Upgrade             bool
	UpgradePathLocation string
}

func NewApplyCommand(tracker *analytics.Tracker) *cobra.Command {
	var cmdEvent analytics.Event

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply the configuration to create or upgrade a battle-tested Kubernetes Fury cluster",
		PreRun: func(cmd *cobra.Command, _ []string) {
			cmdEvent = analytics.NewCommandEvent(cobrax.GetFullname(cmd))
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Get flags.
			flags, err := getCreateClusterCmdFlags(cmd, tracker, cmdEvent)
			if err != nil {
				return err
			}

			// Get home dir.
			logrus.Debug("Getting Home Directory Path...")
			outDir := flags.Outdir

			homeDir, err := os.UserHomeDir()
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while getting user home directory: %w", err)
			}

			if outDir == "" {
				outDir = homeDir
			}

			if flags.BinPath == "" {
				flags.BinPath = filepath.Join(outDir, ".furyctl", "bin")
			}

			if flags.DryRun {
				logrus.Info("Dry run mode enabled, no changes will be applied")
			}

			// Init first half of collaborators.
			client := netx.NewGoGetterClient()
			executor := execx.NewStdExecutor()
			distrodl := distribution.NewDownloader(client, flags.HTTPS)
			depsvl := dependencies.NewValidator(executor, flags.BinPath, flags.FuryctlPath, flags.VpnAutoConnect)

			// Init packages.
			execx.NoTTY = flags.NoTTY

			// Validate base requirements.
			if err := depsvl.ValidateBaseReqs(); err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while validating requirements: %w", err)
			}

			// Download the distribution.
			logrus.Info("Downloading distribution...")
			res, err := distrodl.Download(flags.DistroLocation, flags.FuryctlPath)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while downloading distribution: %w", err)
			}

			cmdEvent.AddClusterDetails(analytics.ClusterDetails{
				Provider:   res.MinimalConf.Kind,
				KFDVersion: res.DistroManifest.Version,
				Phase:      flags.Phase,
				DryRun:     flags.DryRun,
			})

			basePath := filepath.Join(outDir, ".furyctl", res.MinimalConf.Metadata.Name)

			// Init second half of collaborators.
			depsdl := dependencies.NewDownloader(client, basePath, flags.BinPath, flags.HTTPS)

			// Validate the furyctl.yaml file.
			logrus.Info("Validating configuration file...")
			if err := config.Validate(flags.FuryctlPath, res.RepoPath); err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while validating configuration file: %w", err)
			}

			// Download the dependencies.
			if !flags.SkipDepsDownload {
				logrus.Info("Downloading dependencies...")
				if errs, _ := depsdl.DownloadAll(res.DistroManifest); len(errs) > 0 {
					cmdEvent.AddErrorMessage(ErrDownloadDependenciesFailed)
					tracker.Track(cmdEvent)

					return fmt.Errorf("%w: %v", ErrDownloadDependenciesFailed, errs)
				}
			}

			// Validate the dependencies, unless explicitly told to skip it.
			if !flags.SkipDepsValidation {
				logrus.Info("Validating dependencies...")
				if err := depsvl.Validate(res); err != nil {
					cmdEvent.AddErrorMessage(err)
					tracker.Track(cmdEvent)

					return fmt.Errorf("error while validating dependencies: %w", err)
				}
			}

			absFuryctlPath, err := filepath.Abs(flags.FuryctlPath)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while initializing cluster creation: %w", err)
			}

			// Define cluster creation paths.
			paths := cluster.CreatorPaths{
				ConfigPath: absFuryctlPath,
				WorkDir:    basePath,
				DistroPath: res.RepoPath,
				BinPath:    flags.BinPath,
			}

			// Set debug mode.
			execx.Debug = flags.Debug

			// Create the cluster.
			clusterCreator, err := cluster.NewCreator(
				res.MinimalConf,
				res.DistroManifest,
				paths,
				flags.Phase,
				flags.SkipVpn,
				flags.VpnAutoConnect,
				flags.DryRun,
				flags.Force,
				flags.Upgrade,
				flags.UpgradePathLocation,
			)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while initializing cluster creation: %w", err)
			}

			if err := clusterCreator.Create(flags.StartFrom, flags.Timeout); err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while creating cluster: %w", err)
			}

			cmdEvent.AddSuccessMessage("cluster creation succeeded")
			tracker.Track(cmdEvent)

			return nil
		},
	}

	setupCreateClusterCmdFlags(cmd)

	return cmd
}

func getCreateClusterCmdFlags(cmd *cobra.Command, tracker *analytics.Tracker, cmdEvent analytics.Event) (ClusterCmdFlags, error) {
	debug, err := cmdutil.BoolFlag(cmd, "debug", tracker, cmdEvent)
	if err != nil {
		return ClusterCmdFlags{}, fmt.Errorf(WrappedErrMessage, ErrParsingFlag, "debug")
	}

	furyctlPath, err := cmdutil.StringFlag(cmd, "config", tracker, cmdEvent)
	if err != nil {
		return ClusterCmdFlags{}, fmt.Errorf(WrappedErrMessage, ErrParsingFlag, "config")
	}

	distroLocation, err := cmdutil.StringFlag(cmd, "distro-location", tracker, cmdEvent)
	if err != nil {
		return ClusterCmdFlags{}, fmt.Errorf(WrappedErrMessage, ErrParsingFlag, "distro-location")
	}

	phase, err := cmdutil.StringFlag(cmd, "phase", tracker, cmdEvent)
	if err != nil {
		return ClusterCmdFlags{}, fmt.Errorf(WrappedErrMessage, ErrParsingFlag, "phase")
	}

	err = cluster.CheckPhase(phase)
	if err != nil {
		return ClusterCmdFlags{}, fmt.Errorf("%w: %s: %s", ErrParsingFlag, "phase", err.Error())
	}

	startFrom, err := cmdutil.StringFlag(cmd, "start-from", tracker, cmdEvent)
	if err != nil {
		return ClusterCmdFlags{}, fmt.Errorf(WrappedErrMessage, ErrParsingFlag, "start-from")
	}

	if phase != cluster.OperationPhaseAll && startFrom != "" {
		return ClusterCmdFlags{}, fmt.Errorf(
			"%w: %s: cannot use together with phase flag",
			ErrParsingFlag,
			"start-from",
		)
	}

	err = cluster.ValidateOperationPhase(startFrom)
	if err != nil {
		return ClusterCmdFlags{}, fmt.Errorf("%w: %s: %s", ErrParsingFlag, "start-from", err.Error())
	}

	binPath := cmdutil.StringFlagOptional(cmd, "bin-path")

	skipVpn, err := cmdutil.BoolFlag(cmd, "skip-vpn-confirmation", tracker, cmdEvent)
	if err != nil {
		return ClusterCmdFlags{}, fmt.Errorf(WrappedErrMessage, ErrParsingFlag, "skip-vpn-confirmation")
	}

	vpnAutoConnect, err := cmdutil.BoolFlag(cmd, "vpn-auto-connect", tracker, cmdEvent)
	if err != nil {
		return ClusterCmdFlags{}, fmt.Errorf(WrappedErrMessage, ErrParsingFlag, "vpn-auto-connect")
	}

	if skipVpn && vpnAutoConnect {
		return ClusterCmdFlags{}, fmt.Errorf(
			"%w: %s: cannot use together with skip-vpn flag",
			ErrParsingFlag,
			"vpn-auto-connect",
		)
	}

	dryRun, err := cmdutil.BoolFlag(cmd, "dry-run", tracker, cmdEvent)
	if err != nil {
		return ClusterCmdFlags{}, fmt.Errorf(WrappedErrMessage, ErrParsingFlag, "dry-run")
	}

	noTTY, err := cmdutil.BoolFlag(cmd, "no-tty", tracker, cmdEvent)
	if err != nil {
		return ClusterCmdFlags{}, fmt.Errorf(WrappedErrMessage, ErrParsingFlag, "no-tty")
	}

	force, err := cmdutil.BoolFlag(cmd, "force", tracker, cmdEvent)
	if err != nil {
		return ClusterCmdFlags{}, fmt.Errorf(WrappedErrMessage, ErrParsingFlag, "force")
	}

	skipDepsDownload, err := cmdutil.BoolFlag(cmd, "skip-deps-download", tracker, cmdEvent)
	if err != nil {
		return ClusterCmdFlags{}, fmt.Errorf(WrappedErrMessage, ErrParsingFlag, "skip-deps-download")
	}

	skipDepsValidation, err := cmdutil.BoolFlag(cmd, "skip-deps-validation", tracker, cmdEvent)
	if err != nil {
		return ClusterCmdFlags{}, fmt.Errorf(WrappedErrMessage, ErrParsingFlag, "skip-deps-validation")
	}

	https, err := cmdutil.BoolFlag(cmd, "https", tracker, cmdEvent)
	if err != nil {
		return ClusterCmdFlags{}, fmt.Errorf(WrappedErrMessage, ErrParsingFlag, "https")
	}

	timeout, err := cmdutil.IntFlag(cmd, "timeout", tracker, cmdEvent)
	if err != nil {
		return ClusterCmdFlags{}, fmt.Errorf(WrappedErrMessage, ErrParsingFlag, "timeout")
	}

	outdir, err := cmdutil.StringFlag(cmd, "outdir", tracker, cmdEvent)
	if err != nil {
		return ClusterCmdFlags{}, fmt.Errorf(WrappedErrMessage, ErrParsingFlag, "outdir")
	}

	upgrade, err := cmdutil.BoolFlag(cmd, "upgrade", tracker, cmdEvent)
	if err != nil {
		return ClusterCmdFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "upgrade")
	}

	upgradePathLocation, err := cmdutil.StringFlag(cmd, "upgrade-path-location", tracker, cmdEvent)
	if err != nil {
		return ClusterCmdFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "upgrade-path-location")
	}

	return ClusterCmdFlags{
		Debug:               debug,
		FuryctlPath:         furyctlPath,
		DistroLocation:      distroLocation,
		Phase:               phase,
		StartFrom:           startFrom,
		BinPath:             binPath,
		SkipVpn:             skipVpn,
		VpnAutoConnect:      vpnAutoConnect,
		DryRun:              dryRun,
		SkipDepsDownload:    skipDepsDownload,
		SkipDepsValidation:  skipDepsValidation,
		NoTTY:               noTTY,
		Force:               force,
		HTTPS:               https,
		Timeout:             timeout,
		Outdir:              outdir,
		Upgrade:             upgrade,
		UpgradePathLocation: upgradePathLocation,
	}, nil
}

func setupCreateClusterCmdFlags(cmd *cobra.Command) {
	cmd.Flags().StringP(
		"config",
		"c",
		"furyctl.yaml",
		"Path to the configuration file",
	)

	cmd.Flags().StringP(
		"phase",
		"p",
		"",
		"Limit the execution to a specific phase. Options are: infrastructure, kubernetes, distribution, plugins",
	)

	cmd.Flags().String(
		"start-from",
		"",
		"Start the execution from a specific phase. Options are: pre-infrastructure, infrastructure, post-infrastructure, pre-kubernetes, "+
			"kubernetes, post-kubernetes, pre-distribution, distribution, post-distribution, plugins",
	)

	cmd.Flags().StringP(
		"distro-location",
		"",
		"",
		"Location where to download schemas, defaults and the distribution manifests from. "+
			"It can either be a local path (eg: /path/to/fury/distribution) or "+
			"a remote URL (eg: git::git@github.com:sighupio/fury-distribution?depth=1&ref=BRANCH_NAME). "+
			"Any format supported by hashicorp/go-getter can be used.",
	)

	cmd.Flags().StringP(
		"bin-path",
		"b",
		"",
		"Path to the folder where all the dependencies' binaries are installed",
	)

	cmd.Flags().Bool(
		"skip-deps-download",
		false,
		"Skip downloading the distribution modules, installers and binaries",
	)

	cmd.Flags().Bool(
		"skip-deps-validation",
		false,
		"Skip validating dependencies",
	)

	cmd.Flags().Bool(
		"dry-run",
		false,
		"Allows to inspect what resources will be created before applying them",
	)

	cmd.Flags().Bool(
		"vpn-auto-connect",
		false,
		"When set will automatically connect to the created VPN by the infrastructure phase "+
			"(requires OpenVPN installed in the system)",
	)

	cmd.Flags().Bool(
		"skip-vpn-confirmation",
		false,
		"When set will not wait for user confirmation that the VPN is connected",
	)

	cmd.Flags().Bool(
		"force",
		false,
		"WARNING: furyctl won't ask for confirmation and will proceed applying upgrades and reducers",
	)

	cmd.Flags().String(
		"kubeconfig",
		"",
		"Path to the kubeconfig file, mandatory if you want to run the distribution phase alone and the KUBECONFIG environment variable is not set",
	)

	//nolint:gomnd,revive // ignore magic number linters
	cmd.Flags().Int(
		"timeout",
		3600,
		"Timeout in seconds for the whole cluster creation process. Expressed in seconds",
	)

	cmd.Flags().Bool(
		"upgrade",
		false,
		"When set will run the upgrade scripts",
	)

	cmd.Flags().StringP(
		"upgrade-path-location",
		"",
		"",
		"Location where the upgrade scripts are located, if not set the embedded ones will be used",
	)
}
