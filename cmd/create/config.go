// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package create

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	distroConfig "github.com/sighupio/fury-distribution/pkg/config"
	"github.com/sighupio/furyctl/internal/analytics"
	"github.com/sighupio/furyctl/internal/cmd/cmdutil"
	"github.com/sighupio/furyctl/internal/config"
	"github.com/sighupio/furyctl/internal/distribution"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	netx "github.com/sighupio/furyctl/internal/x/net"
)

func NewConfigCmd(furyctlBinVersion string, tracker *analytics.Tracker) *cobra.Command {
	var cmdEvent analytics.Event

	cmd := &cobra.Command{
		Use:   "config",
		Short: "scaffolds a new furyctl config file",
		PreRun: func(cmd *cobra.Command, _ []string) {
			cmdEvent = analytics.NewCommandEvent(cobrax.GetFullname(cmd))
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Get flags.
			debug, err := cmdutil.BoolFlag(cmd, "debug", tracker, cmdEvent)
			if err != nil {
				return fmt.Errorf("%w: %s", ErrParsingFlag, "debug")
			}

			furyctlPath, err := cmdutil.StringFlag(cmd, "config", tracker, cmdEvent)
			if err != nil {
				return fmt.Errorf("%w: config", ErrParsingFlag)
			}

			distroLocation, err := cmdutil.StringFlag(cmd, "distro-location", tracker, cmdEvent)
			if err != nil {
				return fmt.Errorf("%w: %s", ErrParsingFlag, "distro-location")
			}

			version, err := cmdutil.StringFlag(cmd, "version", tracker, cmdEvent)
			if err != nil {
				return fmt.Errorf("%w: version", ErrParsingFlag)
			}

			kind, err := cmdutil.StringFlag(cmd, "kind", tracker, cmdEvent)
			if err != nil {
				return fmt.Errorf("%w: kind", ErrParsingFlag)
			}

			apiVersion, err := cmdutil.StringFlag(cmd, "api-version", tracker, cmdEvent)
			if err != nil {
				return fmt.Errorf("%w: kind", ErrParsingFlag)
			}

			name, err := cmdutil.StringFlag(cmd, "name", tracker, cmdEvent)
			if err != nil {
				return fmt.Errorf("%w: name", ErrParsingFlag)
			}

			minimalConf := distroConfig.Furyctl{
				APIVersion: apiVersion,
				Kind:       kind,
				Metadata: distroConfig.FuryctlMeta{
					Name: name,
				},
				Spec: distroConfig.FuryctlSpec{
					DistributionVersion: version,
				},
			}

			// Init collaborators.
			distrodl := distribution.NewDownloader(netx.NewGoGetterClient())

			// Init packages.
			execx.Debug = debug

			// Download the distribution.
			logrus.Info("Downloading distribution...")
			res, err := distrodl.DoDownload(furyctlBinVersion, distroLocation, minimalConf)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("failed to download distribution: %w", err)
			}

			cmdEvent.AddClusterDetails(analytics.ClusterDetails{
				KFDVersion: res.DistroManifest.Version,
			})

			data := map[string]string{
				"Kind":                kind,
				"Name":                name,
				"DistributionVersion": version,
			}

			out, err := config.Create(res, furyctlPath, cmdEvent, tracker, data)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("failed to create config file: %w", err)
			}

			logrus.Infof("Config file created successfully at: %s", out.Name())

			cmdEvent.AddSuccessMessage(fmt.Sprintf("Config file created successfully at: %s", out.Name()))
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

	cmd.Flags().StringP(
		"version",
		"v",
		"v1.23.3",
		"distribution version to use (eg: v1.23.3)",
	)

	cmd.Flags().StringP(
		"kind",
		"k",
		"EKSCluster",
		"type of cluster to create (eg: EKSCluster)",
	)

	cmd.Flags().StringP(
		"api-version",
		"a",
		"kfd.sighup.io/v1alpha2",
		"version of the api to use for the selected kind (eg: kfd.sighup.io/v1alpha2)",
	)

	cmd.Flags().StringP(
		"name",
		"n",
		"example",
		"name of cluster to create",
	)

	return cmd
}
