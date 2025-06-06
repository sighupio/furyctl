// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package del

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/sighupio/furyctl/internal/analytics"
	"github.com/sighupio/furyctl/internal/app"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/config"
	"github.com/sighupio/furyctl/internal/git"
	"github.com/sighupio/furyctl/internal/lockfile"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
	"github.com/sighupio/furyctl/pkg/dependencies"
	dist "github.com/sighupio/furyctl/pkg/distribution"
	netx "github.com/sighupio/furyctl/pkg/x/net"
)

const WrappedErrMessage = "%w: %s"

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

var (
	ErrParsingFlag                = errors.New("error while parsing flag")
	ErrDownloadDependenciesFailed = errors.New("dependencies download failed")
)

func NewClusterCmd() *cobra.Command {
	var cmdEvent analytics.Event

	clusterCmd := &cobra.Command{
		Use:   "cluster",
		Short: "Deletes a cluster",
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
			flags, err := getDeleteClusterCmdFlags()
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return err
			}

			// Init paths.

			outDir := flags.Outdir

			if flags.DryRun {
				logrus.Info("Dry run mode enabled, no changes will be applied")
			}

			var distrodl *dist.Downloader

			// Init first half of collaborators.
			client := netx.NewGoGetterClient()
			executor := execx.NewStdExecutor()
			depsvl := dependencies.NewValidator(executor, flags.BinPath, flags.FuryctlPath, flags.VpnAutoConnect)

			if flags.DistroLocation == "" {
				distrodl = dist.NewCachingDownloader(client, outDir, flags.GitProtocol, flags.DistroPatchesLocation)
			} else {
				distrodl = dist.NewDownloader(client, flags.GitProtocol, flags.DistroPatchesLocation)
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

			lockFileHandler := lockfile.NewLockFile(res.MinimalConf.Metadata.Name)
			sigs := make(chan os.Signal, 1)

			go func() {
				<-sigs

				if lockFileHandler != nil {
					logrus.Debugf("Removing lock file %s", lockFileHandler.Path)

					if err := lockFileHandler.Remove(); err != nil {
						logrus.Errorf("error while removing lock file %s: %v", lockFileHandler.Path, err)
					}
				}

				os.Exit(1) //nolint:revive // ignore exit code
			}()

			signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

			err = lockFileHandler.Verify()
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while verifying lock file %s: %w", lockFileHandler.Path, err)
			}

			err = lockFileHandler.Create()
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while creating lock file %s: %w", lockFileHandler.Path, err)
			}
			defer lockFileHandler.Remove() //nolint:errcheck // ignore error

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
			} else {
				logrus.Info("Dependencies download skipped")
			}

			// Validate the dependencies, unless explicitly told to skip it.
			if !flags.SkipDepsValidation {
				logrus.Info("Validating dependencies...")
				if err := depsvl.Validate(res); err != nil {
					cmdEvent.AddErrorMessage(err)
					tracker.Track(cmdEvent)

					return fmt.Errorf("error while validating dependencies: %w", err)
				}
			} else {
				logrus.Info("Dependencies validation skipped")
			}

			// Define cluster deletion paths.
			paths := cluster.DeleterPaths{
				ConfigPath: flags.FuryctlPath,
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
				_, err = fmt.Println("\nWARNING: You are about to delete a cluster. This action is irreversible.")
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

	clusterCmd.Flags().StringP(
		"config",
		"c",
		"furyctl.yaml",
		"Path to the configuration file",
	)

	clusterCmd.Flags().StringP(
		"distro-location",
		"",
		"",
		"Location where to download schemas, defaults and the distribution manifests from. "+
			"It can either be a local path (eg: /path/to/distribution) or "+
			"a remote URL (eg: git::git@github.com:sighupio/distribution?depth=1&ref=BRANCH_NAME). "+
			"Any format supported by hashicorp/go-getter can be used",
	)

	clusterCmd.Flags().String(
		"distro-patches",
		"",
		"Location where the distribution's user-made patches can be downloaded from. "+
			"This can be either a local path (eg: /path/to/distro-patches) or "+
			"a remote URL (eg: git::git@github.com:your-org/distro-patches?depth=1&ref=BRANCH_NAME). "+
			"Any format supported by hashicorp/go-getter can be used."+
			" Patches within this location must be in a folder named after the distribution version (eg: v1.29.0) and "+
			"must have the same structure as the distribution's repository",
	)

	clusterCmd.Flags().StringP(
		"bin-path",
		"b",
		"",
		"Path to the folder where all the dependencies' binaries are downloaded",
	)

	clusterCmd.Flags().StringP(
		"phase",
		"p",
		"",
		"Limit execution to a specific phase. Options are: "+strings.Join([]string{
			cluster.OperationPhaseInfrastructure,
			cluster.OperationPhaseKubernetes,
			cluster.OperationPhaseDistribution,
		}, ", "),
	)

	if err := clusterCmd.RegisterFlagCompletionFunc("phase", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{
			cluster.OperationPhaseInfrastructure,
			cluster.OperationPhaseKubernetes,
			cluster.OperationPhaseDistribution,
		}, cobra.ShellCompDirectiveDefault
	}); err != nil {
		logrus.Fatalf("error while registering flag completion: %v", err)
	}

	clusterCmd.Flags().Bool(
		"dry-run",
		false,
		"when set furyctl won't delete any resources. Allows to inspect what resources will be deleted",
	)

	clusterCmd.Flags().Bool(
		"vpn-auto-connect",
		false,
		"When set will automatically connect to the created VPN by the infrastructure phase "+
			"(requires OpenVPN installed in the system)",
	)

	clusterCmd.Flags().Bool(
		"skip-vpn-confirmation",
		false,
		"When set will not wait for user confirmation that the VPN is connected",
	)

	clusterCmd.Flags().Bool(
		"force",
		false,
		"WARNING: furyctl won't ask for confirmation and will force delete the cluster and its resources.",
	)

	clusterCmd.Flags().Bool(
		"skip-deps-download",
		false,
		"Skip downloading the distribution modules, installers and binaries",
	)

	clusterCmd.Flags().Bool(
		"skip-deps-validation",
		false,
		"Skip validating dependencies",
	)

	return clusterCmd
}

func getDeleteClusterCmdFlags() (ClusterCmdFlags, error) {
	var err error

	// The binPath path must be calculated here because when we launch the tools
	// we sometimes change the working directory where the binary is launched
	// breaking the relative path.
	binPath := viper.GetString("bin-path")
	if binPath == "" {
		// The outdir flag is already calculated in the root command, so we can use it here.
		binPath = filepath.Join(viper.GetString("outdir"), ".furyctl", "bin")
	} else {
		binPath, err = filepath.Abs(binPath)
		if err != nil {
			return ClusterCmdFlags{}, fmt.Errorf("error while getting absolute path for bin folder: %w", err)
		}
	}

	distroPatchesLocation := viper.GetString("distro-patches")
	if distroPatchesLocation != "" {
		distroPatchesLocation, err = filepath.Abs(viper.GetString("distro-patches"))
		if err != nil {
			return ClusterCmdFlags{}, fmt.Errorf("error while getting absolute path of distro patches location: %w", err)
		}
	}

	furyctlPath := viper.GetString("config")

	if furyctlPath == "" {
		return ClusterCmdFlags{}, fmt.Errorf("%w --config: cannot be an empty string", ErrParsingFlag)
	}

	furyctlPath, err = filepath.Abs(furyctlPath)
	if err != nil {
		return ClusterCmdFlags{}, fmt.Errorf("error while getting configuration file absolute path: %w", err)
	}

	phase := viper.GetString("phase")
	if err = cluster.CheckPhase(phase); err != nil {
		return ClusterCmdFlags{}, fmt.Errorf("%w: %s: %s", ErrParsingFlag, "phase", err.Error())
	}

	vpnAutoConnect := viper.GetBool("vpn-auto-connect")
	skipVpn := viper.GetBool("skip-vpn-confirmation")

	if skipVpn && vpnAutoConnect {
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

	return ClusterCmdFlags{
		Debug:                 viper.GetBool("debug"),
		FuryctlPath:           furyctlPath,
		DistroLocation:        viper.GetString("distro-location"),
		Phase:                 phase,
		BinPath:               binPath,
		SkipVpn:               skipVpn,
		VpnAutoConnect:        vpnAutoConnect,
		DryRun:                viper.GetBool("dry-run"),
		Force:                 viper.GetBool("force"),
		NoTTY:                 viper.GetBool("no-tty"),
		GitProtocol:           typedGitProtocol,
		Outdir:                viper.GetString("outdir"),
		SkipDepsDownload:      viper.GetBool("skip-deps-download"),
		SkipDepsValidation:    viper.GetBool("skip-deps-validation"),
		DistroPatchesLocation: distroPatchesLocation,
	}, nil
}
