// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/r3labs/diff/v3"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/furyctl/internal/analytics"
	"github.com/sighupio/furyctl/internal/app"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/flags"
	"github.com/sighupio/furyctl/internal/git"
	"github.com/sighupio/furyctl/internal/state"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	"github.com/sighupio/furyctl/pkg/diffs"
	dist "github.com/sighupio/furyctl/pkg/distribution"
	netx "github.com/sighupio/furyctl/pkg/x/net"
	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

type DiffCommandFlags struct {
	Debug                 bool
	FuryctlPath           string
	DistroLocation        string
	Phase                 string
	NoTTY                 bool
	GitProtocol           git.Protocol
	BinPath               string
	Outdir                string
	UpgradePathLocation   string
	DistroPatchesLocation string
}

func NewDiffCmd() *cobra.Command {
	var cmdEvent analytics.Event

	diffCmd := &cobra.Command{
		Use:   "diff",
		Short: "Diff the current configuration with the one in the cluster",
		PreRun: func(cmd *cobra.Command, _ []string) {
			cmdEvent = analytics.NewCommandEvent(cobrax.GetFullname(cmd))

			// Load and validate flags from configuration FIRST.
			if err := flags.LoadAndMergeCommandFlags("diff"); err != nil {
				logrus.Fatalf("failed to load flags from configuration: %v", err)
			}

			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				logrus.Fatalf("error while binding flags: %v", err)
			}
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			ctn := app.GetContainerInstance()

			tracker := ctn.Tracker()
			defer tracker.Flush()

			flags, err := getDiffCommandFlags()
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return err
			}

			execx.Debug = flags.Debug

			client := netx.NewGoGetterClient()

			distrodl := dist.NewDownloader(client, flags.GitProtocol, flags.DistroPatchesLocation)

			if flags.DistroLocation == "" {
				distrodl = dist.NewCachingDownloader(client, flags.Outdir, flags.GitProtocol, flags.DistroPatchesLocation)
			}

			logrus.Info("Downloading distribution...")
			res, err := distrodl.Download(flags.DistroLocation, flags.FuryctlPath)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while downloading distribution: %w", err)
			}

			basePath := filepath.Join(flags.Outdir, ".furyctl", res.MinimalConf.Metadata.Name)

			stateStore := state.NewStore(
				res.RepoPath,
				flags.FuryctlPath,
				basePath,
				res.DistroManifest.Tools.Common.Kubectl.Version,
				flags.BinPath,
			)

			diffChecker, err := createDiffChecker(stateStore, flags.FuryctlPath)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while creating configuration diff checker: %w", err)
			}

			phasePath, err := getPhasePath(
				flags.FuryctlPath,
				basePath,
				res.RepoPath,
				flags.BinPath,
				flags.Phase,
				res.MinimalConf,
				res.DistroManifest,
				flags.UpgradePathLocation,
			)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while getting phase path: %w", err)
			}

			d, err := getDiffs(diffChecker, phasePath)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while getting diffs: %w", err)
			}

			if len(d) > 0 {
				fmt.Printf(
					"Differences found from previous cluster configuration:\n%s",
					diffChecker.DiffToString(d),
				)
			} else {
				logrus.Info("No differences found from previous cluster configuration")
			}

			cmdEvent.AddSuccessMessage("diff command executed successfully")
			tracker.Track(cmdEvent)

			return nil
		},
	}

	diffCmd.Flags().StringP(
		"config",
		"c",
		"furyctl.yaml",
		"Path to the configuration file",
	)

	diffCmd.Flags().StringP(
		"phase",
		"p",
		"",
		"Limit the execution to a specific phase. Options are: "+strings.Join(cluster.MainPhases(), ", "),
	)

	// Add completion for the phase flag.
	if err := diffCmd.RegisterFlagCompletionFunc("phase", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return cluster.MainPhases(), cobra.ShellCompDirectiveDefault
	}); err != nil {
		logrus.Fatalf("error while registering flag completion: %v", err)
	}

	diffCmd.Flags().StringP(
		"distro-location",
		"",
		"",
		"Location where to download schemas, defaults and the distribution manifests from. "+
			"It can either be a local path (eg: /path/to/distribution) or "+
			"a remote URL (eg: git::git@github.com:sighupio/distribution?depth=1&ref=BRANCH_NAME). "+
			"Any format supported by hashicorp/go-getter can be used",
	)

	diffCmd.Flags().String(
		"distro-patches",
		"",
		"Location where the distribution's user-made patches can be downloaded from. "+
			"This can be either a local path (eg: /path/to/distro-patches) or "+
			"a remote URL (eg: git::git@github.com:your-org/distro-patches?depth=1&ref=BRANCH_NAME). "+
			"Any format supported by hashicorp/go-getter can be used."+
			" Patches within this location must be in a folder named after the distribution version (eg: v1.29.0) and "+
			"must have the same structure as the distribution's repository",
	)

	diffCmd.Flags().StringP(
		"bin-path",
		"b",
		"",
		"Path to the folder where all the dependencies' binaries are downloaded",
	)

	diffCmd.Flags().StringP(
		"upgrade-path-location",
		"",
		"",
		"Location where the upgrade scripts are located, if not set the embedded ones will be used",
	)

	return diffCmd
}

func getPhasePath(
	furyctlPath string,
	workDir string,
	distroPath string,
	binPath string,
	phase string,
	minimalConf config.Furyctl,
	distroManifest config.KFD,
	upgradePathLocation string,
) (string, error) {
	absFuryctlPath, err := filepath.Abs(furyctlPath)
	if err != nil {
		return "", fmt.Errorf("error while initializing cluster creation: %w", err)
	}

	paths := cluster.CreatorPaths{
		ConfigPath: absFuryctlPath,
		WorkDir:    workDir,
		DistroPath: distroPath,
		BinPath:    binPath,
	}

	clusterCreator, err := cluster.NewCreator(
		minimalConf,
		distroManifest,
		paths,
		phase,
		true,
		false,
		false,
		true,
		[]string{"all"},
		true,
		upgradePathLocation,
		"",
		[]string{},
	)
	if err != nil {
		return "", fmt.Errorf("error while initializing cluster creator: %w", err)
	}

	phasePath, err := clusterCreator.GetPhasePath(phase)
	if err != nil {
		return "", fmt.Errorf("error while getting phase path: %w", err)
	}

	return phasePath, nil
}

func createDiffChecker(stateStore state.Storer, furyctlPath string) (diffs.Checker, error) {
	var diffChecker diffs.Checker

	storedCfg := map[string]any{}

	storedCfgStr, err := stateStore.GetConfig()
	if err != nil {
		return diffChecker, fmt.Errorf("error while getting current cluster config: %w", err)
	}

	if err := yamlx.UnmarshalV3(storedCfgStr, &storedCfg); err != nil {
		return diffChecker, fmt.Errorf("error while unmarshalling config file: %w", err)
	}

	newCfg, err := yamlx.FromFileV3[map[string]any](furyctlPath)
	if err != nil {
		return diffChecker, fmt.Errorf("error while reading config file: %w", err)
	}

	return diffs.NewBaseChecker(storedCfg, newCfg), nil
}

func getDiffs(diffChecker diffs.Checker, phasePath string) (diff.Changelog, error) {
	changeLog, err := diffChecker.GenerateDiff()
	if err != nil {
		return changeLog, fmt.Errorf("error while generating diff: %w", err)
	}

	return diffChecker.FilterDiffFromPhase(changeLog, phasePath), nil
}

func getDiffCommandFlags() (DiffCommandFlags, error) {
	var err error

	binPath := viper.GetString("bin-path")
	if binPath == "" {
		binPath = filepath.Join(viper.GetString("outdir"), ".furyctl", "bin")
	} else {
		binPath, err = filepath.Abs(binPath)
		if err != nil {
			return DiffCommandFlags{}, fmt.Errorf("error while getting absolute path for bin folder: %w", err)
		}
	}

	distroPatchesLocation := viper.GetString("distro-patches")
	if distroPatchesLocation != "" {
		distroPatchesLocation, err = filepath.Abs(distroPatchesLocation)
		if err != nil {
			return DiffCommandFlags{}, fmt.Errorf("error while getting absolute path of distro patches location: %w", err)
		}
	}

	phase := viper.GetString("phase")
	if err := cluster.CheckPhase(phase); err != nil {
		return DiffCommandFlags{}, fmt.Errorf("%w: %s: %s", ErrParsingFlag, "phase", err.Error())
	}

	gitProtocol := viper.GetString("git-protocol")

	typedGitProtocol, err := git.NewProtocol(gitProtocol)
	if err != nil {
		return DiffCommandFlags{}, fmt.Errorf("%w: %w", ErrParsingFlag, err)
	}

	return DiffCommandFlags{
		Debug:                 viper.GetBool("debug"),
		FuryctlPath:           viper.GetString("config"),
		DistroLocation:        viper.GetString("distro-location"),
		Phase:                 phase,
		NoTTY:                 viper.GetBool("no-tty"),
		GitProtocol:           typedGitProtocol,
		BinPath:               binPath,
		Outdir:                viper.GetString("outdir"),
		UpgradePathLocation:   viper.GetString("upgrade-path-location"),
		DistroPatchesLocation: distroPatchesLocation,
	}, nil
}
