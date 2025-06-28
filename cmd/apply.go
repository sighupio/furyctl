// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/sighupio/furyctl/internal/analytics"
	_ "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/ekscluster"
	"github.com/sighupio/furyctl/internal/app"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/config"
	"github.com/sighupio/furyctl/internal/flags"
	"github.com/sighupio/furyctl/internal/git"
	"github.com/sighupio/furyctl/internal/lockfile"
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

var (
	ErrDownloadDependenciesFailed = errors.New("dependencies download failed")
	ErrPhaseInvalid               = errors.New("phase is not valid")
)

func NewApplyCmd() *cobra.Command {
	var cmdEvent analytics.Event

	applyCmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply the configuration to create, update, or upgrade a battle-tested SIGHUP Distribution cluster",
		Example: `  furyctl apply                                     Apply all the configuration to the cluster using the default configuration file name
  furyctl apply --config mycluster.yaml             Apply a custom configuration file
  furyctl apply --phase distribution                Apply a single phase, for example the distribution phase
  furyctl apply --post-apply-phases distribution    Apply all the phases, and repeat the distribution phase afterwards
`,
		PreRun: func(cmd *cobra.Command, _ []string) {
			cmdEvent = analytics.NewCommandEvent(cobrax.GetFullname(cmd))

			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				logrus.Fatalf("error while binding flags: %v", err)
			}

			// Load and merge flags from configuration file
			configPath := flags.GetConfigPathFromViper()
			flagsManager := flags.NewManager(filepath.Dir(configPath))
			if err := flagsManager.LoadAndMergeFlags(configPath, "apply"); err != nil {
				logrus.Debugf("Failed to load flags from configuration: %v", err)
				// Continue execution - flags loading is optional
			}
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			ctn := app.GetContainerInstance()

			tracker := ctn.Tracker()
			tracker.Flush()

			// Get flags.
			flags, err := getApplyCmdFlags()
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return err
			}

			if flags.DryRun {
				logrus.Info("Dry run mode enabled, no changes will be applied")
			}

			var distrodl *dist.Downloader

			logrus.Debugf("Using configuration file from path %s", flags.FuryctlPath)

			// Init first half of collaborators.
			client := netx.NewGoGetterClient()
			executor := execx.NewStdExecutor()
			depsvl := dependencies.NewValidator(executor, flags.BinPath, flags.FuryctlPath, flags.VpnAutoConnect)

			if flags.DistroLocation == "" {
				distrodl = dist.NewCachingDownloader(client, flags.Outdir, flags.GitProtocol, flags.DistroPatchesLocation)
			} else {
				distrodl = dist.NewDownloader(client, flags.GitProtocol, flags.DistroPatchesLocation)
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

				os.Exit(1) //nolint:revive // ignore error
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

			basePath := filepath.Join(flags.Outdir, ".furyctl", res.MinimalConf.Metadata.Name)

			// Init second half of collaborators.
			depsdl := dependencies.NewCachingDownloader(client, flags.Outdir, basePath, flags.BinPath, flags.GitProtocol)

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

			// Define cluster creation paths.
			paths := cluster.CreatorPaths{
				ConfigPath: flags.FuryctlPath,
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

			cmdEvent.AddSuccessMessage("apply configuration succeeded")
			tracker.Track(cmdEvent)

			return nil
		},
	}

	setupApplyCmdFlags(applyCmd)

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

func getApplyCmdFlags() (ClusterCmdFlags, error) {
	var err error

	skips := getSkipsClusterCmdFlags()

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

	postApplyPhases := viper.GetStringSlice("post-apply-phases")

	if phase != cluster.OperationPhaseAll && len(postApplyPhases) > 0 {
		return ClusterCmdFlags{}, fmt.Errorf("%w: phase and post-apply-phases cannot be used at the same time", ErrParsingFlag)
	}

	if err := validatePostApplyPhasesFlag(postApplyPhases); err != nil {
		return ClusterCmdFlags{}, fmt.Errorf("%w: %s %w", ErrParsingFlag, "post-apply-phases", err)
	}

	return ClusterCmdFlags{
		Debug:          viper.GetBool("debug"),
		FuryctlPath:    furyctlPath,
		DistroLocation: viper.GetString("distro-location"),
		Phase:          phase,
		StartFrom:      startFrom,
		BinPath:        binPath,
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
		DistroPatchesLocation: distroPatchesLocation,
		ClusterSkipsCmdFlags:  skips,
		PostApplyPhases:       postApplyPhases,
	}, nil
}

func validatePostApplyPhasesFlag(phases []string) error {
	for _, phase := range phases {
		if err := cluster.ValidateMainPhases(phase); err != nil {
			return fmt.Errorf("%w: %s", ErrPhaseInvalid, phase)
		}
	}

	return nil
}

func setupApplyCmdFlags(cmd *cobra.Command) {
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
		"Limit the execution to a specific phase. Options are: "+strings.Join(cluster.MainPhases(), ", "),
	)

	// Tab-completion for the "phase" flag.
	if err := cmd.RegisterFlagCompletionFunc("phase", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return cluster.MainPhases(), cobra.ShellCompDirectiveDefault
	}); err != nil {
		logrus.Fatalf("error while registering flag completion: %v", err)
	}

	cmd.Flags().String(
		"start-from",
		"",
		"Start the execution from a specific phase and continue with the following phases. "+
			"Options are: "+strings.Join(slices.Concat(cluster.MainPhases(), cluster.OperationPhases()), ", "))

	// Tab-completion for the "start-from" flag.
	if err := cmd.RegisterFlagCompletionFunc("start-from", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return slices.Concat(cluster.MainPhases(), cluster.OperationPhases()), cobra.ShellCompDirectiveDefault
	}); err != nil {
		logrus.Fatalf("error while registering flag completion: %v", err)
	}

	cmd.Flags().StringP(
		"distro-location",
		"",
		"",
		"Location where to download schemas, defaults, and the distribution manifests from. "+
			"It can either be a local path (eg: /path/to/distribution) or "+
			"a remote URL (eg: git::git@github.com:sighupio/distribution?depth=1&ref=BRANCH_NAME). "+
			"Any format supported by hashicorp/go-getter can be used",
	)

	cmd.Flags().String(
		"distro-patches",
		"",
		"Location where the distribution's user-made patches can be downloaded from. "+
			"This can be either a local path (eg: /path/to/distro-patches) or "+
			"a remote URL (eg: git::git@github.com:your-org/distro-patches?depth=1&ref=BRANCH_NAME). "+
			"Any format supported by hashicorp/go-getter can be used."+
			" Patches within this location must be in a folder named after the distribution version (eg: v1.29.0) and "+
			"must have the same structure as the distribution's repository",
	)

	cmd.Flags().StringP(
		"bin-path",
		"b",
		"",
		"Path to the folder where all the dependencies' binaries are downloaded",
	)

	cmd.Flags().Bool(
		"skip-nodes-upgrade",
		false,
		"On kind OnPremises, this will skip the upgrade of the nodes upgrading only the control-plane",
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
		"WARNING: furyctl won't ask for confirmation and will proceed applying upgrades and migrations. Options are: all, upgrades, migrations, pods-running-check",
	)

	if err := cmd.RegisterFlagCompletionFunc("force", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{
			cluster.ForceFeatureAll,
			cluster.ForceFeatureMigrations,
			cluster.ForceFeaturePodsRunningCheck,
			cluster.ForceFeatureUpgrades,
		}, cobra.ShellCompDirectiveDefault
	}); err != nil {
		logrus.Fatalf("error while registering flag completion: %v", err)
	}

	cmd.Flags().StringSlice(
		"post-apply-phases",
		[]string{},
		"Comma separated list of phases to run after the apply command. Options are: "+strings.Join(cluster.MainPhases(), ", "),
	)

	// Tab-autocomplete for post-apply-phases.
	if err := cmd.RegisterFlagCompletionFunc("post-apply-phases", func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		// The post-apply-phases flag accepts a comma separated list of phases, so we need to take the passed list and add a new valid option at the end of it.
		phases := cluster.MainPhases()
		toCompleteList := strings.Split(toComplete, ",")
		toCompleteLast := toCompleteList[len(toCompleteList)-1]
		completion := []string{}

		for p := range phases {
			if strings.HasPrefix(phases[p], toCompleteLast) {
				toCompleteList[len(toCompleteList)-1] = phases[p]
				completion = append(completion, strings.Join(toCompleteList, ","))
			}
		}

		return completion, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
	}); err != nil {
		logrus.Fatalf("error while registering flag completion: %v", err)
	}

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
		"When set will run the upgrade scripts, allowing to upgrade from one version to another one in the supported upgrade paths. See available target versions with 'get upgrade-paths'",
	)

	cmd.Flags().StringP(
		"upgrade-path-location",
		"",
		"",
		"Set to use a custom location for the upgrade scripts instead of the embedded ones",
	)

	cmd.Flags().String(
		"upgrade-node",
		"",
		"On kind OnPremises, this will upgrade one specific node passed as parameter",
	)
}
