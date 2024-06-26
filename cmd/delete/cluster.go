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

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/internal/analytics"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/cmd/cmdutil"
	"github.com/sighupio/furyctl/internal/config"
	"github.com/sighupio/furyctl/internal/dependencies"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/git"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
	netx "github.com/sighupio/furyctl/internal/x/net"
)

const WrappedErrMessage = "%w: %s"

var (
	ErrParsingFlag                = errors.New("error while parsing flag")
	ErrDownloadDependenciesFailed = errors.New("dependencies download failed")
)

type ClusterCmdFlags struct {
	Debug                 bool
	FuryctlPath           string
	DistroLocation        string
	Phase                 string
	BinPath               string
	Force                 bool
	SkipVpn               bool
	VpnAutoConnect        bool
	DryRun                bool
	NoTTY                 bool
	GitProtocol           git.Protocol
	Outdir                string
	SkipDepsDownload      bool
	SkipDepsValidation    bool
	DistroPatchesLocation string
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

			outDir := flags.Outdir

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

			var distrodl *distribution.Downloader

			// Init first half of collaborators.
			client := netx.NewGoGetterClient()
			executor := execx.NewStdExecutor()
			depsvl := dependencies.NewValidator(executor, flags.BinPath, flags.FuryctlPath, flags.VpnAutoConnect)

			if flags.DistroLocation == "" {
				distrodl = distribution.NewCachingDownloader(client, outDir, flags.GitProtocol, absDistroPatchesLocation)
			} else {
				distrodl = distribution.NewDownloader(client, flags.GitProtocol, absDistroPatchesLocation)
			}

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
				err = fmt.Errorf("error while downloading distribution: %w", err)

				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return err
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

			// Define cluster deletion paths.
			paths := cluster.DeleterPaths{
				ConfigPath: absFuryctlPath,
				WorkDir:    basePath,
				BinPath:    flags.BinPath,
				DistroPath: res.RepoPath,
			}

			// Set debug mode.
			execx.Debug = flags.Debug

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

				prompter := iox.NewPrompter(bufio.NewReader(os.Stdin))

				prompt, err := prompter.Ask("yes")
				if err != nil {
					return fmt.Errorf("error reading user input: %w", err)
				}

				if !prompt {
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
		"Location where to download schemas, defaults and the distribution manifests from. "+
			"It can either be a local path (eg: /path/to/fury/distribution) or "+
			"a remote URL (eg: git::git@github.com:sighupio/fury-distribution?depth=1&ref=BRANCH_NAME). "+
			"Any format supported by hashicorp/go-getter can be used.",
	)

	cmd.Flags().String(
		"distro-patches",
		"",
		"Location where to download distribution's user-made patches from. "+
			cmdutil.AnyGoGetterFormatStr,
	)

	cmd.Flags().StringP(
		"bin-path",
		"b",
		"",
		"Path to the folder where all the dependencies' binaries are installed",
	)

	cmd.Flags().StringP(
		"phase",
		"p",
		"",
		"Limit execution to the specified phase. Options are: infrastructure, kubernetes, distribution",
	)

	cmd.Flags().Bool(
		"dry-run",
		false,
		"when set furyctl won't delete any resources. Allows to inspect what resources will be deleted",
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
		"WARNING: furyctl won't ask for confirmation and will force delete the cluster and its resources.",
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

	return cmd
}

func getDeleteClusterCmdFlags(cmd *cobra.Command, tracker *analytics.Tracker, cmdEvent analytics.Event) (ClusterCmdFlags, error) {
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

	gitProtocol, err := cmdutil.StringFlag(cmd, "git-protocol", tracker, cmdEvent)
	if err != nil {
		return ClusterCmdFlags{}, fmt.Errorf(WrappedErrMessage, ErrParsingFlag, "git-protocol")
	}

	typedGitProtocol, err := git.NewProtocol(gitProtocol)
	if err != nil {
		return ClusterCmdFlags{}, fmt.Errorf("%w: %w", ErrParsingFlag, err)
	}

	outdir, err := cmdutil.StringFlag(cmd, "outdir", tracker, cmdEvent)
	if err != nil {
		return ClusterCmdFlags{}, fmt.Errorf(WrappedErrMessage, ErrParsingFlag, "outdir")
	}

	skipDepsDownload, err := cmdutil.BoolFlag(cmd, "skip-deps-download", tracker, cmdEvent)
	if err != nil {
		return ClusterCmdFlags{}, fmt.Errorf(WrappedErrMessage, ErrParsingFlag, "skip-deps-download")
	}

	skipDepsValidation, err := cmdutil.BoolFlag(cmd, "skip-deps-validation", tracker, cmdEvent)
	if err != nil {
		return ClusterCmdFlags{}, fmt.Errorf(WrappedErrMessage, ErrParsingFlag, "skip-deps-validation")
	}

	distroPatchesLocation, err := cmdutil.StringFlag(cmd, "distro-patches", tracker, cmdEvent)
	if err != nil {
		return ClusterCmdFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "distro-patches")
	}

	return ClusterCmdFlags{
		Debug:                 debug,
		FuryctlPath:           furyctlPath,
		DistroLocation:        distroLocation,
		Phase:                 phase,
		BinPath:               binPath,
		SkipVpn:               skipVpn,
		VpnAutoConnect:        vpnAutoConnect,
		DryRun:                dryRun,
		Force:                 force,
		NoTTY:                 noTTY,
		GitProtocol:           typedGitProtocol,
		Outdir:                outdir,
		SkipDepsDownload:      skipDepsDownload,
		SkipDepsValidation:    skipDepsValidation,
		DistroPatchesLocation: distroPatchesLocation,
	}, nil
}
