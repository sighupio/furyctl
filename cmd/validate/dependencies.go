// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package validate

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
	"github.com/sighupio/furyctl/internal/dependencies/envvars"
	"github.com/sighupio/furyctl/internal/dependencies/tools"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/git"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	netx "github.com/sighupio/furyctl/internal/x/net"
)

var ErrDependencies = errors.New("dependencies are not satisfied")

func NewDependenciesCmd() *cobra.Command {
	var cmdEvent analytics.Event

	dependenciesCmd := &cobra.Command{
		Use:   "dependencies",
		Short: "Validate dependencies for the Kubernetes Fury Distribution version specified in the configuration file",
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
			distroPatchesLocation := viper.GetString("distro-patches")

			outDir := viper.GetString("outdir")
			binPath := viper.GetString("bin-path")
			gitProtocol := viper.GetString("git-protocol")

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

			typedGitProtocol, err := git.NewProtocol(gitProtocol)
			if err != nil {
				return fmt.Errorf("%w: %w", ErrParsingFlag, err)
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
			depsvl := dependencies.NewValidator(executor, "", furyctlPath, false)

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

			// Download the distribution.
			logrus.Info("Downloading distribution...")
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

			toolsValidator := tools.NewValidator(executor, binPath, furyctlPath, false)
			envVarsValidator := envvars.NewValidator()
			errs := make([]error, 0)

			logrus.Info("Validating tools...")

			toks, terrs := toolsValidator.Validate(
				dres.DistroManifest,
				dres.MinimalConf,
			)

			logrus.Info("Validating environment variables...")

			eoks, eerrs := envVarsValidator.Validate(dres.MinimalConf.Kind)

			logrus.Info("Validating tools configuration...")

			errs = append(errs, terrs...)
			errs = append(errs, eerrs...)

			for _, tok := range toks {
				logrus.Infof("%s: binary found in vendor folder", tok)
			}

			for _, eok := range eoks {
				logrus.Infof("%s: environment variable found", eok)
			}

			if len(errs) > 0 {
				logrus.Debugf("Repository path: %s", dres.RepoPath)

				for _, err := range errs {
					logrus.Error(err)
				}

				cmdEvent.AddErrorMessage(ErrDependencies)
				tracker.Track(cmdEvent)

				logrus.Info(
					"You can use the 'furyctl download dependencies' command to download most dependencies, " +
						"and a package manager such as 'asdf' to install the remaining ones.",
				)

				return ErrDependencies
			}

			logrus.Info("Dependencies validation succeeded")

			cmdEvent.AddSuccessMessage("Dependencies validation succeeded")
			tracker.Track(cmdEvent)

			return nil
		},
	}

	dependenciesCmd.Flags().StringP(
		"bin-path",
		"b",
		"",
		"Path to the folder where all the dependencies' binaries are installed",
	)

	dependenciesCmd.Flags().StringP(
		"config",
		"c",
		"furyctl.yaml",
		"Path to the configuration file",
	)

	dependenciesCmd.Flags().StringP(
		"distro-location",
		"",
		"",
		"Location where to download schemas, defaults and the distribution manifests from. "+
			"It can either be a local path (eg: /path/to/fury/distribution) or "+
			"a remote URL (eg: git::git@github.com:sighupio/fury-distribution?depth=1&ref=BRANCH_NAME). "+
			"Any format supported by hashicorp/go-getter can be used.",
	)

	dependenciesCmd.Flags().String(
		"distro-patches",
		"",
		"Location where to download distribution's user-made patches from. "+
			cmdutil.AnyGoGetterFormatStr,
	)

	return dependenciesCmd
}
