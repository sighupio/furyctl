// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package download

import (
	"errors"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/internal/app"
	"github.com/sighupio/furyctl/internal/cobrax"
	"github.com/sighupio/furyctl/internal/netx"
)

var ErrDownloadFailed = fmt.Errorf("dependencies download failed")

func NewDependenciesCmd(furyctlBinVersion string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dependencies",
		Short: "Download dependencies",
		RunE: func(cmd *cobra.Command, _ []string) error {
			debug := cobrax.Flag[bool](cmd, "debug").(bool)
			furyctlPath := cobrax.Flag[string](cmd, "config").(string)
			distroLocation := cobrax.Flag[string](cmd, "distro-location").(string)

			dd := app.NewDownloadDependencies(netx.NewGoGetterClient())

			res, err := dd.Execute(app.DownloadDependenciesRequest{
				FuryctlBinVersion: furyctlBinVersion,
				DistroLocation:    distroLocation,
				FuryctlConfPath:   furyctlPath,
				Debug:             debug,
			})
			if err != nil {
				if !errors.Is(err, app.ErrUnsupportedTools) {
					return err
				}

				logrus.Warnf("unsupported tools: %v", err)
			}

			if res.HasErrors() {
				logrus.Debugf("Repository path: %s", res.RepoPath)

				fmt.Println(res.Error)

				return ErrDownloadFailed
			}

			fmt.Println("dependencies download succeeded")

			return nil
		},
	}

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
