// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package get

import (
	"errors"
	"fmt"
	"os"
	"path"

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
	netx "github.com/sighupio/furyctl/internal/x/net"
)

var (
	ErrParsingFlag                = errors.New("error while parsing flag")
	ErrDownloadDependenciesFailed = errors.New("dependencies download failed")
)

func NewKubeconfigCmd(tracker *analytics.Tracker) *cobra.Command {
	var cmdEvent analytics.Event

	cmd := &cobra.Command{
		Use:   "kubeconfig",
		Short: "Get kubeconfig from a cluster",
		PreRun: func(cmd *cobra.Command, _ []string) {
			cmdEvent = analytics.NewCommandEvent(cobrax.GetFullname(cmd))
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Get flags.
			debug, err := cmdutil.BoolFlag(cmd, "debug", tracker, cmdEvent)
			if err != nil {
				return fmt.Errorf("%w: debug", ErrParsingFlag)
			}

			binPath := cmdutil.StringFlagOptional(cmd, "bin-path")

			furyctlPath, err := cmdutil.StringFlag(cmd, "config", tracker, cmdEvent)
			if err != nil {
				return fmt.Errorf("%w: config", ErrParsingFlag)
			}

			outDir, err := cmdutil.StringFlag(cmd, "outdir", tracker, cmdEvent)
			if err != nil {
				return fmt.Errorf("%w: outdir", ErrParsingFlag)
			}

			distroLocation, err := cmdutil.StringFlag(cmd, "distro-location", tracker, cmdEvent)
			if err != nil {
				return fmt.Errorf("%w: distro-location", ErrParsingFlag)
			}

			gitProtocol, err := cmdutil.StringFlag(cmd, "git-protocol", tracker, cmdEvent)
			if err != nil {
				return fmt.Errorf("%w: git-protocol", ErrParsingFlag)
			}

			skipDepsDownload, err := cmdutil.BoolFlag(cmd, "skip-deps-download", tracker, cmdEvent)
			if err != nil {
				return fmt.Errorf("%w: skip-deps-download", ErrParsingFlag)
			}

			skipDepsValidation, err := cmdutil.BoolFlag(cmd, "skip-deps-validation", tracker, cmdEvent)
			if err != nil {
				return fmt.Errorf("%w: skip-deps-validation", ErrParsingFlag)
			}

			// Get Current dir.
			logrus.Debug("Getting current directory path...")

			currentDir, err := os.Getwd()
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while getting current directory: %w", err)
			}

			// Get home dir.
			logrus.Debug("Getting Home directory path...")
			homeDir, err := os.UserHomeDir()
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while getting user home directory: %w", err)
			}

			if binPath == "" {
				binPath = path.Join(homeDir, ".furyctl", "bin")
			}

			parsedGitProtocol := (git.Protocol)(gitProtocol)

			if outDir == "" {
				outDir = currentDir
			}

			// Init packages.
			execx.Debug = debug

			executor := execx.NewStdExecutor()

			distrodl := &distribution.Downloader{}
			depsvl := dependencies.NewValidator(executor, binPath, furyctlPath, false)

			// Init first half of collaborators.
			client := netx.NewGoGetterClient()

			if distroLocation == "" {
				distrodl = distribution.NewCachingDownloader(client, outDir, parsedGitProtocol, "")
			} else {
				distrodl = distribution.NewDownloader(client, parsedGitProtocol, "")
			}

			// Validate base requirements.
			if err := depsvl.ValidateBaseReqs(); err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while validating requirements: %w", err)
			}

			// Download the distribution.
			logrus.Info("Downloading distribution...")

			res, err := distrodl.Download(distroLocation, furyctlPath)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while downloading distribution: %w", err)
			}

			basePath := path.Join(outDir, ".furyctl", res.MinimalConf.Metadata.Name)

			// Init second half of collaborators.
			depsdl := dependencies.NewCachingDownloader(client, homeDir, basePath, binPath, parsedGitProtocol)

			// Validate the furyctl.yaml file.
			logrus.Info("Validating configuration file...")
			if err := config.Validate(furyctlPath, res.RepoPath); err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while validating configuration file: %w", err)
			}

			// Download the dependencies.
			if !skipDepsDownload {
				logrus.Info("Downloading dependencies...")
				if _, err := depsdl.DownloadTools(res.DistroManifest); err != nil {
					cmdEvent.AddErrorMessage(ErrDownloadDependenciesFailed)
					tracker.Track(cmdEvent)

					return fmt.Errorf("%w: %v", ErrDownloadDependenciesFailed, err)
				}
			}

			// Validate the dependencies, unless explicitly told to skip it.
			if !skipDepsValidation {
				logrus.Info("Validating dependencies...")
				if err := depsvl.Validate(res); err != nil {
					cmdEvent.AddErrorMessage(err)
					tracker.Track(cmdEvent)

					return fmt.Errorf("error while validating dependencies: %w", err)
				}
			}

			getter, err := cluster.NewKubeconfigGetter(res.MinimalConf, res.DistroManifest, res.RepoPath, furyctlPath, outDir)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while creating the kubeconfig getter: %w", err)
			}

			if err := getter.Get(); err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while getting the kubeconfig, please check that the cluster is up and running and is reachable: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringP(
		"bin-path",
		"b",
		"",
		"Path to the folder where all the dependencies' binaries are installed",
	)

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
			cmdutil.AnyGoGetterFormatStr,
	)

	cmd.Flags().Bool(
		"skip-deps-download",
		false,
		"Skip downloading the binaries",
	)

	cmd.Flags().Bool(
		"skip-deps-validation",
		false,
		"Skip validating dependencies",
	)

	return cmd
}
