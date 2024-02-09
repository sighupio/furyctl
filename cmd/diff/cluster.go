// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package diff

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
	"github.com/sighupio/furyctl/internal/git"
	"github.com/sighupio/furyctl/internal/state"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	netx "github.com/sighupio/furyctl/internal/x/net"
)

var ErrParsingFlag = errors.New("error while parsing flag")

type ClusterCommandFlags struct {
	Debug               bool
	FuryctlPath         string
	DistroLocation      string
	Phase               string
	NoTTY               bool
	GitProtocol         git.Protocol
	BinPath             string
	Outdir              string
	UpgradePathLocation string
}

func NewClusterCommand(tracker *analytics.Tracker) *cobra.Command {
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
			distrodl := distribution.NewCachingDownloader(client, flags.GitProtocol)

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

			diffChecker, err := diffs.NewStateChecker(stateStore, flags.FuryctlPath)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while creating diff checker: %w", err)
			}

			schemaSettings := cluster.GetSchemaSettings(res.MinimalConf.APIVersion, res.MinimalConf.Kind)
			phasePath, err := schemaSettings.SchemaPathForPhase(flags.Phase)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while getting phase path: %w", err)
			}

			d, err := diffChecker.GetFilteredDiffs(phasePath)
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

	cmd.Flags().StringP(
		"bin-path",
		"b",
		"",
		"Path to the folder where all the dependencies' binaries are installed",
	)

	cmd.Flags().StringP(
		"upgrade-path-location",
		"",
		"",
		"Location where the upgrade scripts are located, if not set the embedded ones will be used",
	)

	return cmd
}

func getDiffCommandFlags(
	cmd *cobra.Command,
	tracker *analytics.Tracker,
	cmdEvent analytics.Event,
) (ClusterCommandFlags, error) {
	debug, err := cmdutil.BoolFlag(cmd, "debug", tracker, cmdEvent)
	if err != nil {
		return ClusterCommandFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "debug")
	}

	furyctlPath, err := cmdutil.StringFlag(cmd, "config", tracker, cmdEvent)
	if err != nil {
		return ClusterCommandFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "config")
	}

	distroLocation, err := cmdutil.StringFlag(cmd, "distro-location", tracker, cmdEvent)
	if err != nil {
		return ClusterCommandFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "distro-location")
	}

	phase, err := cmdutil.StringFlag(cmd, "phase", tracker, cmdEvent)
	if err != nil {
		return ClusterCommandFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "phase")
	}

	if err := cluster.CheckPhase(phase); err != nil {
		return ClusterCommandFlags{}, fmt.Errorf("%w: %s: %s", ErrParsingFlag, "phase", err.Error())
	}

	noTTY, err := cmdutil.BoolFlag(cmd, "no-tty", tracker, cmdEvent)
	if err != nil {
		return ClusterCommandFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "no-tty")
	}

	binPath := cmdutil.StringFlagOptional(cmd, "bin-path")

	outdir, err := cmdutil.StringFlag(cmd, "outdir", tracker, cmdEvent)
	if err != nil {
		return ClusterCommandFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "outdir")
	}

	gitProtocol, err := cmdutil.StringFlag(cmd, "git-protocol", tracker, cmdEvent)
	if err != nil {
		return ClusterCommandFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "git-protocol")
	}

	typedGitProtocol, err := git.NewProtocol(gitProtocol)
	if err != nil {
		return ClusterCommandFlags{}, fmt.Errorf("%w: %w", ErrParsingFlag, err)
	}

	upgradePathLocation, err := cmdutil.StringFlag(cmd, "upgrade-path-location", tracker, cmdEvent)
	if err != nil {
		return ClusterCommandFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "upgrade-path-location")
	}

	return ClusterCommandFlags{
		Debug:               debug,
		FuryctlPath:         furyctlPath,
		DistroLocation:      distroLocation,
		Phase:               phase,
		NoTTY:               noTTY,
		GitProtocol:         typedGitProtocol,
		BinPath:             binPath,
		Outdir:              outdir,
		UpgradePathLocation: upgradePathLocation,
	}, nil
}
