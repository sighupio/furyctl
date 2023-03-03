// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package del

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/internal/analytics"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/cmd/cmdutil"
	"github.com/sighupio/furyctl/internal/dependencies"
	"github.com/sighupio/furyctl/internal/distribution"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	netx "github.com/sighupio/furyctl/internal/x/net"
)

var (
	ErrParsingFlag        = errors.New("error while parsing flag")
	ErrKubeconfigReq      = errors.New("when running distribution phase, either the KUBECONFIG environment variable or the --kubeconfig flag should be set")
	ErrKubeconfigNotFound = errors.New("kubeconfig file not found")
)

type ClusterCmdFlags struct {
	Debug          bool
	FuryctlPath    string
	DistroLocation string
	Phase          string
	BinPath        string
	Force          bool
	SkipVpn        bool
	VpnAutoConnect bool
	DryRun         bool
	NoTTY          bool
	Kubeconfig     string
}

func NewClusterCmd(tracker *analytics.Tracker) *cobra.Command {
	var cmdEvent analytics.Event

	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "Deletes a cluster",
		PreRun: func(cmd *cobra.Command, _ []string) {
			cmdEvent = analytics.NewCommandEvent(cobrax.GetFullname(cmd))
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Get flags.
			flags, err := getDeleteClusterCmdFlags(cmd, tracker, cmdEvent)
			if err != nil {
				return err
			}

			// Init paths.
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("error while getting user home directory: %w", err)
			}

			kubeconfigPath := flags.Kubeconfig

			// Check if kubeconfig is needed.
			if flags.Phase == cluster.OperationPhaseDistribution || flags.Phase == cluster.OperationPhaseAll {
				if kubeconfigPath == "" {
					kubeconfigFromEnv := os.Getenv("KUBECONFIG")

					if kubeconfigFromEnv == "" {
						return ErrKubeconfigReq
					}

					kubeconfigPath = kubeconfigFromEnv

					logrus.Warnf("Missing --kubeconfig flag, falling back to KUBECONFIG from environment: %s", kubeconfigFromEnv)
				}

				kubeAbsPath, err := filepath.Abs(kubeconfigPath)
				if err != nil {
					return fmt.Errorf("error while getting absolute path of kubeconfig: %w", err)
				}

				kubeconfigPath = kubeAbsPath

				// Check the kubeconfig file exists.
				if _, err := os.Stat(kubeconfigPath); os.IsNotExist(err) {
					return fmt.Errorf("%w in %s", ErrKubeconfigNotFound, kubeconfigPath)
				}
			}

			if flags.BinPath == "" {
				flags.BinPath = filepath.Join(homeDir, ".furyctl", "bin")
			}

			// Init first half of collaborators.
			client := netx.NewGoGetterClient()
			executor := execx.NewStdExecutor()
			distrodl := distribution.NewDownloader(client)

			execx.Debug = flags.Debug
			execx.NoTTY = flags.NoTTY

			// Download the distribution.
			logrus.Info("Downloading distribution...")
			res, err := distrodl.Download(flags.DistroLocation, flags.FuryctlPath)
			if err != nil {
				err = fmt.Errorf("error while downloading distribution: %w", err)

				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return err
			}

			basePath := filepath.Join(homeDir, ".furyctl", res.MinimalConf.Metadata.Name)

			// Init second half of collaborators.
			depsvl := dependencies.NewValidator(executor, flags.BinPath, flags.FuryctlPath)

			// Validate the dependencies.
			logrus.Info("Validating dependencies...")
			if err := depsvl.Validate(res); err != nil {
				return fmt.Errorf("error while validating dependencies: %w", err)
			}

			// Define cluster deletion paths.
			paths := cluster.DeleterPaths{
				ConfigPath: flags.FuryctlPath,
				WorkDir:    basePath,
				BinPath:    flags.BinPath,
				Kubeconfig: kubeconfigPath,
			}

			clusterDeleter, err := cluster.NewDeleter(
				res.MinimalConf,
				res.DistroManifest,
				paths,
				flags.Phase,
				flags.SkipVpn,
				flags.VpnAutoConnect,
				flags.DryRun,
			)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while initializing cluster deleter: %w", err)
			}

			if !flags.Force {
				_, err = fmt.Println("WARNING: You are about to delete a cluster. This action is irreversible.")
				if err != nil {
					cmdEvent.AddErrorMessage(err)
					tracker.Track(cmdEvent)

					return fmt.Errorf("error while printing to stdout: %w", err)
				}

				_, err = fmt.Println("Are you sure you want to continue? Only 'yes' will be accepted to confirm.")
				if err != nil {
					cmdEvent.AddErrorMessage(err)
					tracker.Track(cmdEvent)

					return fmt.Errorf("error while printing to stdout: %w", err)
				}

				if !askForConfirmation() {
					return nil
				}
			}

			err = clusterDeleter.Delete()
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while deleting cluster: %w", err)
			}

			cmdEvent.AddSuccessMessage("Cluster deleted successfully!")
			tracker.Track(cmdEvent)

			return nil
		},
	}

	cmd.Flags().StringP(
		"config",
		"c",
		"furyctl.yaml",
		"Path to the configuration file",
	)

	cmd.Flags().StringP(
		"distro-location",
		"",
		"",
		"Base URL used to download schemas, defaults and the distribution manifest. "+
			"It can either be a local path(eg: /path/to/fury/distribution) or "+
			"a remote URL(eg: git::git@github.com:sighupio/fury-distribution?depth=1&ref=BRANCH_NAME)."+
			"Any format supported by hashicorp/go-getter can be used.",
	)

	cmd.Flags().StringP(
		"bin-path",
		"b",
		"",
		"Path to the bin folder where all dependencies are installed",
	)

	cmd.Flags().StringP(
		"phase",
		"p",
		"",
		"Limit execution to the specified phase",
	)

	cmd.Flags().Bool(
		"dry-run",
		false,
		"when set furyctl won't delete any resources. Allows to inspect what resources will be deleted",
	)

	cmd.Flags().Bool(
		"vpn-auto-connect",
		false,
		"When set will automatically connect to the created VPN in the infrastructure phase",
	)

	cmd.Flags().Bool(
		"skip-vpn",
		false,
		"When set will not wait for user confirmation to connect to the VPN",
	)

	cmd.Flags().Bool(
		"force",
		false,
		"WARNING: furyctl won't ask for confirmation and will force delete the cluster and it resources.",
	)

	cmd.Flags().String(
		"kubeconfig",
		"",
		"Path to the kubeconfig file, mandatory if you want to run the distribution phase alone or "+
			"if you want to delete a cluster and the KUBECONFIG environment variable is not set",
	)

	return cmd
}

func getDeleteClusterCmdFlags(cmd *cobra.Command, tracker *analytics.Tracker, cmdEvent analytics.Event) (ClusterCmdFlags, error) {
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

	err = cluster.CheckPhase(phase)
	if err != nil {
		return ClusterCmdFlags{}, fmt.Errorf("%w: %s: %s", ErrParsingFlag, "phase", err.Error())
	}

	binPath := cmdutil.StringFlagOptional(cmd, "bin-path")

	skipVpn, err := cmdutil.BoolFlag(cmd, "skip-vpn", tracker, cmdEvent)
	if err != nil {
		return ClusterCmdFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "skip-vpn")
	}

	vpnAutoConnect, err := cmdutil.BoolFlag(cmd, "vpn-auto-connect", tracker, cmdEvent)
	if err != nil {
		return ClusterCmdFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "vpn-auto-connect")
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
		return ClusterCmdFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "dry-run")
	}

	noTTY, err := cmdutil.BoolFlag(cmd, "no-tty", tracker, cmdEvent)
	if err != nil {
		return ClusterCmdFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "no-tty")
	}

	kubeconfig, err := cmdutil.StringFlag(cmd, "kubeconfig", tracker, cmdEvent)
	if err != nil {
		return ClusterCmdFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "kubeconfig")
	}

	force, err := cmdutil.BoolFlag(cmd, "force", tracker, cmdEvent)
	if err != nil {
		return ClusterCmdFlags{}, fmt.Errorf("%w: force", ErrParsingFlag)
	}

	return ClusterCmdFlags{
		Debug:          debug,
		FuryctlPath:    furyctlPath,
		DistroLocation: distroLocation,
		Phase:          phase,
		BinPath:        binPath,
		SkipVpn:        skipVpn,
		VpnAutoConnect: vpnAutoConnect,
		DryRun:         dryRun,
		Force:          force,
		NoTTY:          noTTY,
		Kubeconfig:     kubeconfig,
	}, nil
}

func askForConfirmation() bool {
	reader := bufio.NewReader(os.Stdin)

	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	response = strings.TrimSuffix(response, "\n")

	return strings.Compare(response, "yes") == 0
}
