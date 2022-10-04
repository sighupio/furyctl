// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package validate

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/internal/dependencies/envvars"
	"github.com/sighupio/furyctl/internal/dependencies/tools"
	"github.com/sighupio/furyctl/internal/distribution"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	netx "github.com/sighupio/furyctl/internal/x/net"
)

var ErrDependencies = fmt.Errorf("dependencies are not satisfied")

func NewDependenciesCmd(furyctlBinVersion string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dependencies",
		Short: "Validate furyctl.yaml file",
		RunE: func(cmd *cobra.Command, _ []string) error {
			debug := cobrax.Flag[bool](cmd, "debug").(bool)
			binPath := cobrax.Flag[string](cmd, "bin-path").(string)
			furyctlPath := cobrax.Flag[string](cmd, "config").(string)
			distroLocation := cobrax.Flag[string](cmd, "distro-location").(string)

			dloader := distribution.NewDownloader(netx.NewGoGetterClient(), debug)

			dres, err := dloader.Download(furyctlBinVersion, distroLocation, furyctlPath)
			if err != nil {
				return err
			}

			toolsValidator := tools.NewValidator(execx.NewStdExecutor(), binPath)
			envVarsValidator := envvars.NewValidator()
			errs := make([]error, 0)

			errs = append(errs, toolsValidator.Validate(dres.DistroManifest)...)
			errs = append(errs, envVarsValidator.Validate(dres.MinimalConf.Kind)...)

			if len(errs) > 0 {
				logrus.Debugf("Repository path: %s", dres.RepoPath)

				for _, err := range errs {
					logrus.Error(err)
				}

				return ErrDependencies
			}

			logrus.Info("Dependencies validation succeeded")

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
