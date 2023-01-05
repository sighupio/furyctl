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
	"github.com/sighupio/furyctl/internal/cmd/cmdutil"
	"github.com/sighupio/furyctl/internal/config"
	"github.com/sighupio/furyctl/internal/dependencies"
	"github.com/sighupio/furyctl/internal/distribution"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	netx "github.com/sighupio/furyctl/internal/x/net"
)

var (
	ErrParsingFlag                = errors.New("error while parsing flag")
	ErrDownloadDependenciesFailed = errors.New("dependencies download failed")
	ErrKubeconfigReq              = errors.New("when running distribution phase alone, either the KUBECONFIG environment variable or the --kubeconfig flag should be set")
)

type ClusterCmdFlags struct {
	Debug              bool
	FuryctlPath        string
	DistroLocation     string
	Phase              string
	SkipPhase          string
	BinPath            string
	VpnAutoConnect     bool
	DryRun             bool
	SkipDepsDownload   bool
	SkipDepsValidation bool
	Kubeconfig         string
}

func NewClusterCmd(version string, tracker *analytics.Tracker) *cobra.Command {
	var cmdEvent analytics.Event

	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "Creates a battle-tested Kubernetes Fury cluster",
		PreRun: func(cmd *cobra.Command, _ []string) {
			cmdEvent = analytics.NewCommandEvent(cobrax.GetFullname(cmd))
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Get flags.
			flags, err := getCmdFlags(cmd, tracker, cmdEvent)
			if err != nil {
				return err
			}

			// Init paths.
			homeDir, err := os.UserHomeDir()
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while getting current working directory: %w", err)
			}

			// Check if kubeconfig is needed.
			if flags.Phase == "distribution" || flags.SkipPhase == "kubernetes" {
				if flags.Kubeconfig == "" {
					kubeconfigFromEnv := os.Getenv("KUBECONFIG")

					if kubeconfigFromEnv == "" {
						return ErrKubeconfigReq
					}

					logrus.Warnf("Missing --kubeconfig flag, falling back to KUBECONFIG from environment: %s", kubeconfigFromEnv)
				}
			}

			if flags.BinPath == "" {
				flags.BinPath = filepath.Join(homeDir, ".furyctl", "bin")
			}

			// Init first half of collaborators.
			client := netx.NewGoGetterClient()
			executor := execx.NewStdExecutor()
			distrodl := distribution.NewDownloader(client)

			// Init packages.
			execx.Debug = flags.Debug

			// Download the distribution.
			logrus.Info("Downloading distribution...")
			res, err := distrodl.Download(version, flags.DistroLocation, flags.FuryctlPath)

			cmdEvent.AddClusterDetails(analytics.ClusterDetails{
				Provider:   res.DistroManifest.Kubernetes.Eks.Version,
				KFDVersion: res.DistroManifest.Version,
				Phase:      flags.Phase,
			})

			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while downloading distribution: %w", err)
			}

			basePath := filepath.Join(homeDir, ".furyctl", res.MinimalConf.Metadata.Name)

			// Init second half of collaborators.
			depsdl := dependencies.NewDownloader(client, basePath, flags.BinPath)
			depsvl := dependencies.NewValidator(executor, flags.BinPath)

			// Validate the furyctl.yaml file.
			logrus.Info("Validating furyctl.yaml file...")
			if err := config.Validate(flags.FuryctlPath, res.RepoPath); err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while validating furyctl.yaml file: %w", err)
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

			// Auto connect to the VPN if doing complete cluster creation or skipping distribution phase.
			if flags.Phase == "" || flags.SkipPhase == "distribution" {
				flags.VpnAutoConnect = true
			}

			// Define cluster creation paths.
			paths := cluster.CreatorPaths{
				ConfigPath: flags.FuryctlPath,
				WorkDir:    basePath,
				DistroPath: res.RepoPath,
				BinPath:    flags.BinPath,
				Kubeconfig: flags.Kubeconfig,
			}

			// Create the cluster.
			clusterCreator, err := cluster.NewCreator(
				res.MinimalConf,
				res.DistroManifest,
				paths,
				flags.Phase,
				flags.VpnAutoConnect,
			)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while initializing cluster creation: %w", err)
			}

			logrus.Info("Creating cluster...")
			if err := clusterCreator.Create(flags.DryRun, flags.SkipPhase); err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while creating cluster: %w", err)
			}

			if !flags.DryRun && flags.Phase == "" {
				logrus.Info("Cluster created successfully!")
			}

			if flags.Phase != "" {
				logrus.Infof("Phase %s executed successfully!", flags.Phase)
			}

			cmdEvent.AddSuccessMessage("cluster creation succeeded")
			tracker.Track(cmdEvent)

			return nil
		},
	}

	setupClusterCmdFlags(cmd)

	return cmd
}

func getCmdFlags(cmd *cobra.Command, tracker *analytics.Tracker, cmdEvent analytics.Event) (ClusterCmdFlags, error) {
	debug, err := cmdutil.BoolFlag(cmd, "debug", tracker, cmdEvent)
	if err != nil {
		return ClusterCmdFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "debug")
	}

	furyctlPath, err := cmdutil.StringFlag(cmd, "config", tracker, cmdEvent)
	if err != nil {
		return ClusterCmdFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "config")
	}

	distroLocation, err := cmdutil.StringFlag(cmd, "distro-location", tracker, cmdEvent)
	if err != nil {
		return ClusterCmdFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "distro-location")
	}

	phase, err := cmdutil.StringFlag(cmd, "phase", tracker, cmdEvent)
	if err != nil {
		return ClusterCmdFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "phase")
	}

	skipPhase, err := cmdutil.StringFlag(cmd, "skip-phase", tracker, cmdEvent)
	if err != nil {
		return ClusterCmdFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "skip-phase")
	}

	binPath := cmdutil.StringFlagOptional(cmd, "bin-path")

	vpnAutoConnect, err := cmdutil.BoolFlag(cmd, "vpn-auto-connect", tracker, cmdEvent)
	if err != nil {
		return ClusterCmdFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "vpn-auto-connect")
	}

	dryRun, err := cmdutil.BoolFlag(cmd, "dry-run", tracker, cmdEvent)
	if err != nil {
		return ClusterCmdFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "dry-run")
	}

	skipDepsDownload, err := cmdutil.BoolFlag(cmd, "skip-deps-download", tracker, cmdEvent)
	if err != nil {
		return ClusterCmdFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "skip-deps-download")
	}

	skipDepsValidation, err := cmdutil.BoolFlag(cmd, "skip-deps-validation", tracker, cmdEvent)
	if err != nil {
		return ClusterCmdFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "skip-deps-validation")
	}

	kubeconfig, err := cmdutil.StringFlag(cmd, "kubeconfig", tracker, cmdEvent)
	if err != nil {
		return ClusterCmdFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "kubeconfig")
	}

	return ClusterCmdFlags{
		Debug:              debug,
		FuryctlPath:        furyctlPath,
		DistroLocation:     distroLocation,
		Phase:              phase,
		SkipPhase:          skipPhase,
		BinPath:            binPath,
		VpnAutoConnect:     vpnAutoConnect,
		DryRun:             dryRun,
		SkipDepsDownload:   skipDepsDownload,
		SkipDepsValidation: skipDepsValidation,
		Kubeconfig:         kubeconfig,
	}, nil
}

func setupClusterCmdFlags(cmd *cobra.Command) {
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
		"Limit the execution to a specific phase. options are: infrastructure, kubernetes, distribution",
	)

	cmd.Flags().String(
		"skip-phase",
		"",
		"Avoid executing a unwanted phase. options are: infrastructure, kubernetes, distribution. More specifically:\n"+
			"- skipping infrastructure will execute kubernetes and distribution\n"+
			"- skipping kubernetes will only execute distribution\n"+
			"- skipping distribution will execute infrastructure and kubernetes\n",
	)

	cmd.Flags().StringP(
		"distro-location",
		"",
		"",
		"Location where to download schemas, defaults and the distribution manifest. "+
			"It can either be a local path(eg: /path/to/fury/distribution) or "+
			"a remote URL(eg: https://git@github.com/sighupio/fury-distribution?ref=BRANCH_NAME). "+
			"Any format supported by hashicorp/go-getter can be used.",
	)

	cmd.Flags().StringP(
		"bin-path",
		"b",
		"",
		"Path to the bin folder where all dependencies are installed",
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
		"When set will automatically connect to the created VPN in the infrastructure phase",
	)

	cmd.Flags().String(
		"kubeconfig",
		"",
		"Path to the kubeconfig file, mandatory if you want to run the distribution phase and the KUBECONFIG environment variable is not set",
	)
}
