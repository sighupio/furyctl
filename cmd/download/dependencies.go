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
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
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
		Short: "Download all dependencies from the Fury Distribution specified in the config file",
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

			binPath := cmdutil.StringFlagOptional(cmd, "bin-path")

			homeDir, err := os.UserHomeDir()
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while getting user home directory: %w", err)
			}

			if binPath == "" {
				binPath = filepath.Join(homeDir, ".furyctl", "bin")
			}

			logrus.Info("Downloading dependencies...")

			client := netx.NewGoGetterClient()

			distrodl := distribution.NewDownloader(client)

			dres, err := distrodl.Download(distroLocation, furyctlPath)
			cmdEvent.AddClusterDetails(analytics.ClusterDetails{
				KFDVersion: dres.DistroManifest.Version,
			})

			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("failed to download distribution: %w", err)
			}

			basePath := filepath.Join(homeDir, ".furyctl", dres.MinimalConf.Metadata.Name)

			depsdl := dependencies.NewDownloader(client, basePath, binPath)

			errs, uts := depsdl.DownloadAll(dres.DistroManifest)

			for _, ut := range uts {
				logrus.Warn(fmt.Sprintf("'%s' download is not supported, please install it manually", ut))
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
		"Path to the bin folder where all dependencies are installed",
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
		"Base URL used to download schemas, defaults and the distribution manifest. "+
			"It can either be a local path(eg: /path/to/fury/distribution) or "+
			"a remote URL(eg: git::git@github.com:sighupio/fury-distribution?depth=1&ref=BRANCH_NAME)."+
			"Any format supported by hashicorp/go-getter can be used.",
	)

	return cmd
}
