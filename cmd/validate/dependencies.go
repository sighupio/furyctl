// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package validate

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/internal/analytics"
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

var ErrDependencies = fmt.Errorf("dependencies are not satisfied")

func NewDependenciesCmd(tracker *analytics.Tracker) *cobra.Command {
	var cmdEvent analytics.Event

	cmd := &cobra.Command{
		Use:   "dependencies",
		Short: "Validate dependencies for the Kubernetes Fury Distribution version specified in the configuration file",
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

			binPath := cobrax.Flag[string](cmd, "bin-path")
			if binPath == "" {
				// Init paths.
				logrus.Debug("Getting Home Directory Path...")
				outDir, err := cmdutil.StringFlag(cmd, "outdir", tracker, cmdEvent)
				if err != nil {
					return fmt.Errorf("%w: outdir", ErrParsingFlag)
				}

				homeDir, err := os.UserHomeDir()
				if err != nil {
					cmdEvent.AddErrorMessage(err)
					tracker.Track(cmdEvent)

					return fmt.Errorf("error while getting user home directory: %w", err)
				}

				if outDir == "" {
					outDir = homeDir
				}

				binPath = filepath.Join(outDir, ".furyctl", "bin")
			}

			gitProtocol, err := cmdutil.StringFlag(cmd, "git-protocol", tracker, cmdEvent)
			if err != nil {
				return fmt.Errorf("%w: git-protocol", ErrParsingFlag)
			}

			typedGitProtocol, err := git.NewProtocol(gitProtocol)
			if err != nil {
				return fmt.Errorf("%w: %w", ErrParsingFlag, err)
			}

			dloader := distribution.NewDownloader(netx.NewGoGetterClient(), typedGitProtocol)
			executor := execx.NewStdExecutor()
			depsvl := dependencies.NewValidator(executor, "", furyctlPath, false)

			// Validate base requirements.
			if err := depsvl.ValidateBaseReqs(); err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while validating requirements: %w", err)
			}

			// Download the distribution.
			logrus.Info("Downloading distribution...")
			dres, err := dloader.Download(distroLocation, furyctlPath)
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

	return cmd
}
