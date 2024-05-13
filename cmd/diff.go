// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/r3labs/diff/v3"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/furyctl/internal/analytics"
	"github.com/sighupio/furyctl/internal/app"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/diffs"
	"github.com/sighupio/furyctl/internal/git"
	"github.com/sighupio/furyctl/internal/state"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
	dist "github.com/sighupio/furyctl/pkg/distribution"
	netx "github.com/sighupio/furyctl/pkg/x/net"
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
				return err
			}

			execx.Debug = flags.Debug

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

			absDistroPatchesLocation := flags.DistroPatchesLocation

			if absDistroPatchesLocation != "" {
				absDistroPatchesLocation, err = filepath.Abs(flags.DistroPatchesLocation)
				if err != nil {
					cmdEvent.AddErrorMessage(err)
					tracker.Track(cmdEvent)

					return fmt.Errorf("error while getting absolute path of distro patches location: %w", err)
				}
			}

			client := netx.NewGoGetterClient()

			distrodl := dist.NewDownloader(client, flags.GitProtocol, absDistroPatchesLocation)

			if flags.DistroLocation == "" {
				distrodl = dist.NewCachingDownloader(client, outDir, flags.GitProtocol, absDistroPatchesLocation)
			}

			logrus.Info("Downloading distribution...")
			res, err := distrodl.Download(flags.DistroLocation, flags.FuryctlPath)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while downloading distribution: %w", err)
			}

			basePath := filepath.Join(outDir, ".furyctl", res.MinimalConf.Metadata.Name)

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

				return fmt.Errorf("error while creating diff checker: %w", err)
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
		"Limit the execution to a specific phase. Options are: infrastructure, kubernetes, distribution",
	)

	diffCmd.Flags().StringP(
		"distro-location",
		"",
		"",
		"Location where to download schemas, defaults and the distribution manifests from. "+
			"It can either be a local path (eg: /path/to/fury/distribution) or "+
			"a remote URL (eg: git::git@github.com:sighupio/fury-distribution?depth=1&ref=BRANCH_NAME). "+
			"Any format supported by hashicorp/go-getter can be used.",
	)

	diffCmd.Flags().String(
		"distro-patches",
		"",
		"Location where to download distribution's user-made patches from. "+
			"Any format supported by hashicorp/go-getter can be used.",
	)

	diffCmd.Flags().StringP(
		"bin-path",
		"b",
		"",
		"Path to the folder where all the dependencies' binaries are installed",
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
		BinPath:               viper.GetString("bin-path"),
		Outdir:                viper.GetString("outdir"),
		UpgradePathLocation:   viper.GetString("upgrade-path-location"),
		DistroPatchesLocation: viper.GetString("distro-patches"),
	}, nil
}
