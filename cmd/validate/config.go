// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package validate

import (
	"errors"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/internal/analytics"
	"github.com/sighupio/furyctl/internal/config"
	"github.com/sighupio/furyctl/internal/distribution"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
	netx "github.com/sighupio/furyctl/internal/x/net"
)

var (
	ErrValidationFailed = fmt.Errorf("config validation failed")
	ErrParsingFlag      = errors.New("error while parsing flag")
)

func NewConfigCmd(furyctlBinVersion string, tracker *analytics.Tracker) *cobra.Command {
	var cmdEvent analytics.Event

	cmd := &cobra.Command{
		Use:   "config",
		Short: "Validate furyctl.yaml file",
		PreRun: func(cmd *cobra.Command, args []string) {
			cmdEvent = analytics.NewCommandEvent(cobrax.GetFullname(cmd))
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			furyctlPath, ok := cobrax.Flag[string](cmd, "config").(string)
			if !ok {
				err := fmt.Errorf("%w: config", ErrParsingFlag)

				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return err
			}
			distroLocation, ok := cobrax.Flag[string](cmd, "distro-location").(string)
			if !ok {
				err := fmt.Errorf("%w: distro-location", ErrParsingFlag)

				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return err
			}

			dloader := distribution.NewDownloader(netx.NewGoGetterClient())

			res, err := dloader.Download(furyctlBinVersion, distroLocation, furyctlPath)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("failed to download distribution: %w", err)
			}

			cmdEvent.AddClusterDetails(analytics.ClusterDetails{
				KFDVersion: res.DistroManifest.Version,
			})

			if err := config.Validate(furyctlPath, res.RepoPath); err != nil {
				logrus.Debugf("Repository path: %s", res.RepoPath)

				logrus.Error(err)

				cmdEvent.AddErrorMessage(ErrValidationFailed)
				tracker.Track(cmdEvent)

				return ErrValidationFailed
			}

			logrus.Info("config validation succeeded")

			cmdEvent.AddSuccessMessage("config validation succeeded")
			tracker.Track(cmdEvent)

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
