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
	"github.com/spf13/viper"

	"github.com/sighupio/furyctl/internal/analytics"
	"github.com/sighupio/furyctl/internal/app"
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
	cmdEvent          analytics.Event   //nolint:gochecknoglobals // needed for cobra/viper compatibility.
	DependenciesCmd   = &cobra.Command{ //nolint:gochecknoglobals // needed for cobra/viper compatibility.
		Use:   "dependencies",
		Short: "Download all dependencies for the Fury Distribution version specified in the configuration file",
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

			furyctlPath := viper.GetString("config")
			distroLocation := viper.GetString("distro-location")
			gitProtocol := viper.GetString("git-protocol")
			outDir := viper.GetString("outdir")
			distroPatchesLocation := viper.GetString("distro-patches")
			binPath := viper.GetString("bin-path")

			typedGitProtocol, err := git.NewProtocol(gitProtocol)
			if err != nil {
				return fmt.Errorf("%w: %w", ErrParsingFlag, err)
			}

			// Init paths.
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

			absDistroPatchesLocation := distroPatchesLocation

			if absDistroPatchesLocation != "" {
				absDistroPatchesLocation, err = filepath.Abs(distroPatchesLocation)
				if err != nil {
					cmdEvent.AddErrorMessage(err)
					tracker.Track(cmdEvent)

					return fmt.Errorf("error while getting absolute path of distro patches location: %w", err)
				}
			}

			var distrodl *distribution.Downloader

			client := netx.NewGoGetterClient()
			executor := execx.NewStdExecutor()
			depsvl := dependencies.NewValidator(executor, binPath, furyctlPath, false)

			if distroLocation == "" {
				distrodl = distribution.NewCachingDownloader(client, outDir, typedGitProtocol, absDistroPatchesLocation)
			} else {
				distrodl = distribution.NewDownloader(client, typedGitProtocol, absDistroPatchesLocation)
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
)

//nolint:gochecknoinits // this pattern requires init function to work.
func init() {
	DependenciesCmd.Flags().StringP(
		"bin-path",
		"b",
		"",
		"Path to the folder where all the dependencies' binaries are installed",
	)

	DependenciesCmd.Flags().StringP(
		"config",
		"c",
		"furyctl.yaml",
		"Path to the configuration file",
	)

	DependenciesCmd.Flags().StringP(
		"distro-location",
		"",
		"",
		"Location where to download schemas, defaults and the distribution manifests from. "+
			"It can either be a local path (eg: /path/to/fury/distribution) or "+
			"a remote URL (eg: git::git@github.com:sighupio/fury-distribution?depth=1&ref=BRANCH_NAME). "+
			"Any format supported by hashicorp/go-getter can be used.",
	)

	DependenciesCmd.Flags().String(
		"distro-patches",
		"",
		"Location where to download distribution's user-made patches from. "+
			cmdutil.AnyGoGetterFormatStr,
	)
}
