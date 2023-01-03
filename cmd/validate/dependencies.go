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
	"github.com/sighupio/furyctl/internal/dependencies/envvars"
	"github.com/sighupio/furyctl/internal/dependencies/tools"
	"github.com/sighupio/furyctl/internal/distribution"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	netx "github.com/sighupio/furyctl/internal/x/net"
)

var ErrDependencies = fmt.Errorf("dependencies are not satisfied")

func NewDependenciesCmd(tracker *analytics.Tracker) *cobra.Command {
	var cmdEvent analytics.Event

	cmd := &cobra.Command{
		Use:   "dependencies",
		Short: "Validate furyctl.yaml file",
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

			dloader := distribution.NewDownloader(netx.NewGoGetterClient())

			dres, err := dloader.Download(distroLocation, furyctlPath)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("failed to download distribution: %w", err)
			}

			cmdEvent.AddClusterDetails(analytics.ClusterDetails{
				KFDVersion: dres.DistroManifest.Version,
			})

			binPath := cobrax.Flag[string](cmd, "bin-path").(string) //nolint:errcheck,forcetypeassert // optional flag
			if binPath == "" {
				homeDir, err := os.UserHomeDir()
				if err != nil {
					return fmt.Errorf("error while getting user home directory: %w", err)
				}

				binPath = filepath.Join(homeDir, ".furyctl", "bin")
			}

			toolsValidator := tools.NewValidator(execx.NewStdExecutor(), binPath)
			envVarsValidator := envvars.NewValidator()
			errs := make([]error, 0)

			toks, terrs := toolsValidator.Validate(dres.DistroManifest)
			eoks, eerrs := envVarsValidator.Validate(dres.MinimalConf.Kind)

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
						"and a package manager such as 'asdf' to install the other ones.",
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
		"Path to the bin folder where all dependencies are installed",
	)

	cmd.Flags().StringP(
		"config",
		"c",
		"furyctl.yaml",
		"Path to the furyctl.yaml file",
	)

	cmd.Flags().StringP(
		"distro-location",
		"",
		"",
		"Base URL used to download schemas, defaults and the distribution manifest. "+
			"It can either be a local path(eg: /path/to/fury/distribution) or "+
			"a remote URL(eg: https://git@github.com/sighupio/fury-distribution?ref=BRANCH_NAME)."+
			"Any format supported by hashicorp/go-getter can be used.",
	)

	return cmd
}
