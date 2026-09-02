// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"fmt"
	"net"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/sighupio/furyctl/internal/analytics"
	"github.com/sighupio/furyctl/internal/app"
	"github.com/sighupio/furyctl/internal/drydock"
	"github.com/sighupio/furyctl/internal/flags"
	"github.com/sighupio/furyctl/internal/git"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
)

func NewDrydockCmd() *cobra.Command {
	var cmdEvent analytics.Event

	cmd := &cobra.Command{
		Args:  cobra.NoArgs,
		Use:   "drydock",
		Short: "Open a local web wizard that writes a furyctl.yaml step by step",
		Long: `Starts a local web server and opens the browser on a guided wizard.
The wizard asks about the cluster, generates the node list from roles and counts,
suggests environment variables and files for secrets, validates the result against
the schema of the chosen distribution version and writes it to --output.`,
		Example: "furyctl drydock\nfuryctl drydock --port 9090 --distro-location ../distribution --output ./prod/furyctl.yaml",
		PreRun: func(cmd *cobra.Command, _ []string) {
			cmdEvent = analytics.NewCommandEvent(cobrax.GetFullname(cmd))
			ctn := app.GetContainerInstance()

			tracker := ctn.Tracker()
			tracker.Flush()

			// Bind the flags first: a flag on the command line has precedence over the configuration file.
			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				logrus.Fatalf("error while binding flags: %v", err)
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)
			}

			if err := flags.LoadAndMergeCommandFlags("drydock"); err != nil {
				logrus.Fatalf("failed to load flags from configuration: %v", err)
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)
			}
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctn := app.GetContainerInstance()
			tracker := ctn.Tracker()
			defer tracker.Flush()

			address := viper.GetString("address")
			port := viper.GetString("port")

			protocol, err := git.ParseProtocol(viper.GetString("git-protocol"))
			if err != nil {
				return fmt.Errorf("%w: %w", ErrParsingFlag, err)
			}

			reg, err := drydock.LoadEmbedded()
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("loading wizards: %w", err)
			}

			srv := drydock.NewServer(reg, viper.GetString("distro-location"), viper.GetString("output"), protocol)

			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			url := "http://" + net.JoinHostPort(address, port)

			if !viper.GetBool("no-browser") {
				if err := drydock.OpenBrowser(ctx, url); err != nil {
					logrus.Warnf("could not open the browser, open %s yourself: %v", url, err)
				}
			}

			if err := drydock.Run(ctx, address, port, srv); err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("running drydock: %w", err)
			}

			cmdEvent.AddSuccessMessage("drydock session ended")
			tracker.Track(cmdEvent)

			return nil
		},
	}

	cmd.Flags().StringP("address", "a", "127.0.0.1", "Address to listen on")
	cmd.Flags().StringP("port", "p", "8080", "Port to listen on")
	cmd.Flags().String("distro-location", "", "Local path or URL of the distribution to use instead of downloading it")
	cmd.Flags().String("output", "furyctl.yaml", "Path of the configuration file to write")
	cmd.Flags().String("git-protocol", "https", "Protocol used to download the distribution, ssh or https")
	cmd.Flags().Bool("no-browser", false, "Do not open the browser automatically")

	return cmd
}
