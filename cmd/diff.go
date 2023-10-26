// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/r3labs/diff/v3"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/furyctl/internal/analytics"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/cmd/cmdutil"
	"github.com/sighupio/furyctl/internal/diffs"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/state"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	netx "github.com/sighupio/furyctl/internal/x/net"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

var (
	ErrParsingFlag        = errors.New("error while parsing flag")
	ErrKubeconfigReq      = errors.New("either the KUBECONFIG environment variable or the --kubeconfig flag should be set")
	ErrKubeconfigNotFound = errors.New("kubeconfig file not found")
)

type DiffCommandFlags struct {
	Debug          bool
	FuryctlPath    string
	DistroLocation string
	Phase          string
	NoTTY          bool
	HTTPS          bool
	Kubeconfig     string
	BinPath        string
	Outdir         string
}

func NewDiffCommand(tracker *analytics.Tracker) *cobra.Command {
	var cmdEvent analytics.Event

	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Diff the current configuration with the one in the cluster",
		PreRun: func(cmd *cobra.Command, _ []string) {
			cmdEvent = analytics.NewCommandEvent(cobrax.GetFullname(cmd))
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			flags, err := getDiffCommandFlags(cmd, tracker, cmdEvent)
			if err != nil {
				return err
			}

			execx.Debug = flags.Debug

			kubeconfigPath := flags.Kubeconfig

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

			if _, err := os.Stat(kubeconfigPath); os.IsNotExist(err) {
				return fmt.Errorf("%w in %s", ErrKubeconfigNotFound, kubeconfigPath)
			}

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

			client := netx.NewGoGetterClient()
			distrodl := distribution.NewDownloader(client, flags.HTTPS)

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
				kubeconfigPath,
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
				kubeconfigPath,
				flags.Phase,
				res.MinimalConf,
				res.DistroManifest,
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
		"Limit the execution to a specific phase. Options are: infrastructure, kubernetes, distribution",
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
		"kubeconfig",
		"",
		"Path to the kubeconfig file, mandatory if you want to run the distribution phase alone and the KUBECONFIG environment variable is not set",
	)

	cmd.Flags().StringP(
		"bin-path",
		"b",
		"",
		"Path to the folder where all the dependencies' binaries are installed",
	)

	return cmd
}

func getPhasePath(
	furyctlPath string,
	workDir string,
	distroPath string,
	binPath string,
	kubeconfigPath string,
	phase string,
	minimalConf config.Furyctl,
	distroManifest config.KFD,
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
		Kubeconfig: kubeconfigPath,
	}

	clusterCreator, err := cluster.NewCreator(
		minimalConf,
		distroManifest,
		paths,
		phase,
		true,
		false,
		true,
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

func getDiffCommandFlags(
	cmd *cobra.Command,
	tracker *analytics.Tracker,
	cmdEvent analytics.Event,
) (DiffCommandFlags, error) {
	debug, err := cmdutil.BoolFlag(cmd, "debug", tracker, cmdEvent)
	if err != nil {
		return DiffCommandFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "debug")
	}

	furyctlPath, err := cmdutil.StringFlag(cmd, "config", tracker, cmdEvent)
	if err != nil {
		return DiffCommandFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "config")
	}

	distroLocation, err := cmdutil.StringFlag(cmd, "distro-location", tracker, cmdEvent)
	if err != nil {
		return DiffCommandFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "distro-location")
	}

	phase, err := cmdutil.StringFlag(cmd, "phase", tracker, cmdEvent)
	if err != nil {
		return DiffCommandFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "phase")
	}

	err = cluster.CheckPhase(phase)
	if err != nil {
		return DiffCommandFlags{}, fmt.Errorf("%w: %s: %s", ErrParsingFlag, "phase", err.Error())
	}

	kubeconfig, err := cmdutil.StringFlag(cmd, "kubeconfig", tracker, cmdEvent)
	if err != nil {
		return DiffCommandFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "kubeconfig")
	}

	noTTY, err := cmdutil.BoolFlag(cmd, "no-tty", tracker, cmdEvent)
	if err != nil {
		return DiffCommandFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "no-tty")
	}

	binPath := cmdutil.StringFlagOptional(cmd, "bin-path")

	outdir, err := cmdutil.StringFlag(cmd, "outdir", tracker, cmdEvent)
	if err != nil {
		return DiffCommandFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "outdir")
	}

	https, err := cmdutil.BoolFlag(cmd, "https", tracker, cmdEvent)
	if err != nil {
		return DiffCommandFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "https")
	}

	return DiffCommandFlags{
		Debug:          debug,
		FuryctlPath:    furyctlPath,
		DistroLocation: distroLocation,
		Phase:          phase,
		NoTTY:          noTTY,
		HTTPS:          https,
		Kubeconfig:     kubeconfig,
		BinPath:        binPath,
		Outdir:         outdir,
	}, nil
}
