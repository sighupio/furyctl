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
	"github.com/spf13/viper"

	"github.com/sighupio/furyctl/internal/analytics"
	_ "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/ekscluster"
	"github.com/sighupio/furyctl/internal/app"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/config"
	"github.com/sighupio/furyctl/internal/git"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	"github.com/sighupio/furyctl/pkg/dependencies"
	dist "github.com/sighupio/furyctl/pkg/distribution"
	netx "github.com/sighupio/furyctl/pkg/x/net"
)

const WrappedErrMessage = "%w: %s"

type Timeouts struct {
	ProcessTimeout         int
	PodRunningCheckTimeout int
}

type ClusterSkipsCmdFlags struct {
	SkipVpn            bool
	SkipDepsDownload   bool
	SkipDepsValidation bool
	SkipNodesUpgrade   bool
}

type ClusterCmdFlags struct {
	Timeouts
	Debug                 bool
	FuryctlPath           string
	DistroLocation        string
	Phase                 string
	StartFrom             string
	BinPath               string
	VpnAutoConnect        bool
	DryRun                bool
	NoTTY                 bool
	GitProtocol           git.Protocol
	Force                 []string
	Outdir                string
	Upgrade               bool
	UpgradePathLocation   string
	UpgradeNode           string
	DistroPatchesLocation string
	PostApplyPhases       []string
	ClusterSkipsCmdFlags
}

var ErrDownloadDependenciesFailed = errors.New("dependencies download failed")

func NewApplyCmd() *cobra.Command {
	var cmdEvent analytics.Event

	applyCmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply the configuration to create or upgrade a battle-tested Kubernetes Fury cluster",
		PreRun: func(cmd *cobra.Command, _ []string) {
			cmdEvent = analytics.NewCommandEvent(cobrax.GetFullname(cmd))

			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				logrus.Fatalf("error while binding flags: %v", err)
			}
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			ctn := app.GetContainerInstance()

			tracker := ctn.Tracker()
			tracker.Flush()

			// Get flags.
			flags, err := getCreateClusterCmdFlags()
			if err != nil {
				return err
			}

			outDir := flags.Outdir

			// Get home dir.
			logrus.Debug("Getting Home Directory Path...")

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

			absDistroPatchesLocation := flags.DistroPatchesLocation

			if absDistroPatchesLocation != "" {
				absDistroPatchesLocation, err = filepath.Abs(flags.DistroPatchesLocation)
				if err != nil {
					cmdEvent.AddErrorMessage(err)
					tracker.Track(cmdEvent)

					return fmt.Errorf("error while getting absolute path of distro patches location: %w", err)
				}
			}

			var distrodl *dist.Downloader

			// Init first half of collaborators.
			client := netx.NewGoGetterClient()
			executor := execx.NewStdExecutor()
			depsvl := dependencies.NewValidator(executor, flags.BinPath, flags.FuryctlPath, flags.VpnAutoConnect)

			if flags.DistroLocation == "" {
				distrodl = dist.NewCachingDownloader(client, outDir, flags.GitProtocol, absDistroPatchesLocation)
			} else {
				distrodl = dist.NewDownloader(client, flags.GitProtocol, absDistroPatchesLocation)
			}

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
			depsdl := dependencies.NewCachingDownloader(client, outDir, basePath, flags.BinPath, flags.GitProtocol)

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
				flags.SkipNodesUpgrade,
				flags.DryRun,
				flags.Force,
				flags.Upgrade,
				flags.UpgradePathLocation,
				flags.UpgradeNode,
				flags.PostApplyPhases,
			)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while initializing cluster creation: %w", err)
			}

			if err := clusterCreator.Create(
				flags.StartFrom,
				flags.Timeouts.ProcessTimeout,
				flags.PodRunningCheckTimeout,
			); err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while creating cluster: %w", err)
			}

			cmdEvent.AddSuccessMessage("cluster creation succeeded")
			tracker.Track(cmdEvent)

			return nil
		},
	}

	setupCreateClusterCmdFlags(applyCmd)

	return applyCmd
}

func getSkipsClusterCmdFlags() ClusterSkipsCmdFlags {
	return ClusterSkipsCmdFlags{
		SkipVpn:            viper.GetBool("skip-vpn-confirmation"),
		SkipDepsDownload:   viper.GetBool("skip-deps-download"),
		SkipDepsValidation: viper.GetBool("skip-deps-validation"),
		SkipNodesUpgrade:   viper.GetBool("skip-nodes-upgrade"),
	}
}

func getCreateClusterCmdFlags() (ClusterCmdFlags, error) {
	skips := getSkipsClusterCmdFlags()

	phase := viper.GetString("phase")

	if err := cluster.CheckPhase(phase); err != nil {
		return ClusterCmdFlags{}, fmt.Errorf("%w: %s: %s", ErrParsingFlag, "phase", err.Error())
	}

	startFrom := viper.GetString("start-from")

	if phase != cluster.OperationPhaseAll && startFrom != "" {
		return ClusterCmdFlags{}, fmt.Errorf(
			"%w: %s: cannot use together with phase flag",
			ErrParsingFlag,
			"start-from",
		)
	}

	if err := cluster.ValidateOperationPhase(startFrom); err != nil {
		return ClusterCmdFlags{}, fmt.Errorf("%w: %s: %s", ErrParsingFlag, "start-from", err.Error())
	}

	vpnAutoConnect := viper.GetBool("vpn-auto-connect")

	if skips.SkipVpn && vpnAutoConnect {
		return ClusterCmdFlags{}, fmt.Errorf(
			"%w: %s: cannot use together with skip-vpn flag",
			ErrParsingFlag,
			"vpn-auto-connect",
		)
	}

	gitProtocol := viper.GetString("git-protocol")

	typedGitProtocol, err := git.NewProtocol(gitProtocol)
	if err != nil {
		return ClusterCmdFlags{}, fmt.Errorf("%w: %w", ErrParsingFlag, err)
	}

	upgrade := viper.GetBool("upgrade")

	upgradeNode := viper.GetString("upgrade-node")

	if upgrade && upgradeNode != "" {
		return ClusterCmdFlags{}, fmt.Errorf(
			"%w: %s: cannot use together with upgrade flag",
			ErrParsingFlag,
			"upgrade-node",
		)
	}

	return ClusterCmdFlags{
		Debug:          viper.GetBool("debug"),
		FuryctlPath:    viper.GetString("config"),
		DistroLocation: viper.GetString("distro-location"),
		Phase:          phase,
		StartFrom:      startFrom,
		BinPath:        viper.GetString("bin-path"),
		VpnAutoConnect: vpnAutoConnect,
		DryRun:         viper.GetBool("dry-run"),
		NoTTY:          viper.GetBool("no-tty"),
		Force:          viper.GetStringSlice("force"),
		GitProtocol:    typedGitProtocol,
		Timeouts: Timeouts{
			ProcessTimeout:         viper.GetInt("timeout"),
			PodRunningCheckTimeout: viper.GetInt("pod-running-check-timeout"),
		},
		Outdir:                viper.GetString("outdir"),
		Upgrade:               upgrade,
		UpgradePathLocation:   viper.GetString("upgrade-path-location"),
		UpgradeNode:           upgradeNode,
		DistroPatchesLocation: viper.GetString("distro-patches"),
		ClusterSkipsCmdFlags:  skips,
		PostApplyPhases:       viper.GetStringSlice("post-apply-phases"),
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

	cmd.Flags().String(
		"distro-patches",
		"",
		"Location where the distribution's user-made patches can be downloaded from. "+
			"This can be either a local path (eg: /path/to/distro-patches) or "+
			"a remote URL (eg: git::git@github.com:your-org/distro-patches?depth=1&ref=BRANCH_NAME). "+
			"Any format supported by hashicorp/go-getter can be used."+
			" Patches within this location must be in a folder named after the distribution version (eg: v1.29.0) and "+
			"must have the same structure as the distribution's repository.",
	)

	cmd.Flags().StringP(
		"bin-path",
		"b",
		"",
		"Path to the folder where all the dependencies' binaries are installed",
	)

	cmd.Flags().Bool(
		"skip-nodes-upgrade",
		false,
		"On kind OnPremises this will skip the upgrade of the nodes",
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

	cmd.Flags().StringSlice(
		"force",
		[]string{},
		"WARNING: furyctl won't ask for confirmation and will proceed applying upgrades and reducers. Options are: all, upgrades, migrations, pods-running-check",
	)

	cmd.Flags().StringSlice(
		"post-apply-phases",
		[]string{},
		"Phases to run after the apply command. Options are: infrastructure, kubernetes, distribution, plugins",
	)

	cmd.Flags().Int(
		"timeout",
		3600, //nolint:mnd,revive // ignore magic number linters
		"Timeout for the whole cluster creation process, expressed in seconds",
	)

	cmd.Flags().Int(
		"pod-running-check-timeout",
		300, //nolint:mnd,revive // ignore magic number linters
		"Timeout for the pod running check after the worker nodes upgrade, expressed in seconds",
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

	cmd.Flags().String(
		"upgrade-node",
		"",
		"On kind OnPremises this will upgrade a specific node",
	)
}
