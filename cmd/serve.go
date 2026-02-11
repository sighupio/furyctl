// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/sighupio/furyctl/cmd/serve"
	"github.com/sighupio/furyctl/internal/analytics"
	"github.com/sighupio/furyctl/internal/app"
	"github.com/sighupio/furyctl/internal/flags"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
)

func NewServeCmd() *cobra.Command {
	var cmdEvent analytics.Event

	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "Start HTTP server to serve assets from a custom path for the Immutable OS machines bootstrap",
		PreRun: func(cmd *cobra.Command, _ []string) {
			cmdEvent = analytics.NewCommandEvent(cobrax.GetFullname(cmd))
			ctn := app.GetContainerInstance()

			tracker := ctn.Tracker()
			tracker.Flush()

			// Load and validate flags from configuration FIRST.
			if err := flags.LoadAndMergeCommandFlags("serve"); err != nil {
				logrus.Fatalf("failed to load flags from configuration: %v", err)
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)
			}

			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				logrus.Fatalf("error while binding flags: %v", err)
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)
			}
		},

		RunE: func(_ *cobra.Command, _ []string) error {
			return serve.Path(viper.GetString("address"), viper.GetString("port"), viper.GetString("path"))
		},
	}

	serveCmd.Flags().StringP("address", "a", "0.0.0.0", "Address to listen on")
	serveCmd.Flags().StringP("port", "p", "8080", "Port to listen on")
	serveCmd.Flags().StringP("path", "x", "./", "Path to serve assets from")

	return serveCmd
}
