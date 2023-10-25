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

			// Check the kubeconfig file exists.
			if _, err := os.Stat(kubeconfigPath); os.IsNotExist(err) {
				return fmt.Errorf("%w in %s", ErrKubeconfigNotFound, kubeconfigPath)
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

			client := netx.NewGoGetterClient()
			distrodl := distribution.NewDownloader(client, flags.HTTPS)

			// Download the distribution.
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
				Kubeconfig: kubeconfigPath,
			}

			// Set debug mode.
			execx.Debug = flags.Debug

			// Create the cluster.
			clusterCreator, err := cluster.NewCreator(
				res.MinimalConf,
				res.DistroManifest,
				paths,
				flags.Phase,
				true,
				false,
				true,
			)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while initializing cluster creator: %w", err)
			}

			phasePath, err := clusterCreator.GetPhasePath(flags.Phase)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while getting phase path: %w", err)
			}

			storedCfgStr, err := stateStore.GetConfig()
			if err != nil {
				return fmt.Errorf("error while getting current cluster config: %w", err)
			}

			storedCfg := map[string]any{}

			if err := yamlx.UnmarshalV3(storedCfgStr, &storedCfg); err != nil {
				return fmt.Errorf("error while unmarshalling config file: %w", err)
			}

			newCfg, err := yamlx.FromFileV3[map[string]any](flags.FuryctlPath)
			if err != nil {
				return fmt.Errorf("error while reading config file: %w", err)
			}

			diffChecker := diffs.NewBaseChecker(storedCfg, newCfg)

			d, err := diffChecker.GenerateDiff()
			if err != nil {
				return fmt.Errorf("error while generating diff: %w", err)
			}

			d = diffChecker.FilterDiffFromPhase(d, phasePath)

			if len(d) > 0 {
				logrus.Infof(
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
