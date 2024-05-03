// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package download

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/internal/analytics"
	"github.com/sighupio/furyctl/internal/cmd/cmdutil"
	"github.com/sighupio/furyctl/internal/dependencies"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/git"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	netx "github.com/sighupio/furyctl/internal/x/net"
)

var (
	ErrParsingFlag    = errors.New("error while parsing flag")
	ErrDownloadFailed = errors.New("dependencies download failed")
)

func NewDependenciesCmd(tracker *analytics.Tracker) *cobra.Command {
	var cmdEvent analytics.Event

	cmd := &cobra.Command{
		Use:   "dependencies",
		Short: "Download all dependencies for the Fury Distribution version specified in the configuration file",
		PreRun: func(cmd *cobra.Command, _ []string) {
			cmdEvent = analytics.NewCommandEvent(cobrax.GetFullname(cmd))
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			furyctlPath, err := cmdutil.StringFlag(cmd, "config", tracker, cmdEvent)
			if err != nil {
				return fmt.Errorf("%w: config", ErrParsingFlag)
			}

			distroLocation, err := cmdutil.StringFlag(cmd, "distro-location", tracker, cmdEvent)
			if err != nil {
				return fmt.Errorf("%w: distro-location", ErrParsingFlag)
			}

			gitProtocol, err := cmdutil.StringFlag(cmd, "git-protocol", tracker, cmdEvent)
			if err != nil {
				return fmt.Errorf("%w: git-protocol", ErrParsingFlag)
			}

			typedGitProtocol, err := git.NewProtocol(gitProtocol)
			if err != nil {
				return fmt.Errorf("%w: %w", ErrParsingFlag, err)
			}

			distroPatchesLocation, err := cmdutil.StringFlag(cmd, "distro-patches", tracker, cmdEvent)
			if err != nil {
				return fmt.Errorf("%w: %s", ErrParsingFlag, "distro-patches")
			}

			binPath := cmdutil.StringFlagOptional(cmd, "bin-path")

			// Init paths.

			outDir, err := cmdutil.StringFlag(cmd, "outdir", tracker, cmdEvent)
			if err != nil {
				return fmt.Errorf("%w: outdir", ErrParsingFlag)
			}

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

			if binPath == "" {
				binPath = filepath.Join(outDir, ".furyctl", "bin")
			}

			var distrodl *distribution.Downloader

			client := netx.NewGoGetterClient()
			executor := execx.NewStdExecutor()
			depsvl := dependencies.NewValidator(executor, binPath, furyctlPath, false)

			if distroLocation == "" {
				distrodl = distribution.NewCachingDownloader(client, outDir, typedGitProtocol, distroPatchesLocation)
			} else {
				distrodl = distribution.NewDownloader(client, typedGitProtocol, distroPatchesLocation)
			}

			// Validate base requirements.
			if err := depsvl.ValidateBaseReqs(); err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while validating requirements: %w", err)
			}

			dres, err := distrodl.Download(distroLocation, furyctlPath)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("failed to download distribution: %w", err)
			}

			cmdEvent.AddClusterDetails(analytics.ClusterDetails{
				Provider:   dres.MinimalConf.Kind,
				KFDVersion: dres.DistroManifest.Version,
			})

			basePath := filepath.Join(outDir, ".furyctl", dres.MinimalConf.Metadata.Name)

			depsdl := dependencies.NewCachingDownloader(client, outDir, basePath, binPath, typedGitProtocol)

			logrus.Info("Downloading dependencies...")

			errs, uts := depsdl.DownloadAll(dres.DistroManifest)

			for _, ut := range uts {
				logrus.Warn(fmt.Sprintf("'%s' download is not supported, please install it manually if not present", ut))
			}

			if len(errs) > 0 {
				logrus.Debugf("Repository path: %s", dres.RepoPath)

				for _, err := range errs {
					logrus.Error(err)
				}

				cmdEvent.AddErrorMessage(ErrDownloadFailed)
				tracker.Track(cmdEvent)

				return ErrDownloadFailed
			}

			logrus.Info("Dependencies download succeeded")

			cmdEvent.AddSuccessMessage("Dependencies download succeeded")
			tracker.Track(cmdEvent)

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
			"Any format supported by hashicorp/go-getter can be used.",
	)

	cmd.Flags().String(
		"distro-patches",
		"",
		"Location where to download distribution's user-made patches from. "+
			cmdutil.AnyGoGetterFormatStr,
	)

	return cmd
}
