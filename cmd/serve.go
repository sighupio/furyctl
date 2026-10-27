// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/sighupio/furyctl/cmd/serve"
	"github.com/sighupio/furyctl/internal/flags"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	statusx "github.com/sighupio/furyctl/internal/x/status"
)

func NewServeCmd() *cobra.Command {
	serveCmd := &cobra.Command{
		Args:  cobra.NoArgs,
		Use:   "serve",
		Short: "Start HTTP server to serve assets from a custom path for the Immutable OS machines bootstrap",
		PreRun: func(cmd *cobra.Command, _ []string) {
			// Bind the flags first: a flag on the command line has precedence over the configuration file.
			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				logrus.Fatalf("error while binding flags: %v", err)
			}

			if err := flags.LoadAndMergeCommandFlags("serve"); err != nil {
				logrus.Fatalf("failed to load flags from configuration: %v", err)
			}
		},

		RunE: func(_ *cobra.Command, _ []string) error {
			nodes := map[string]string{}

			statusx.Start("serve", execx.NoTTY)

			if err := serve.Path(viper.GetString("address"), viper.GetString("port"), viper.GetString("path"), nodes); err != nil {
				return err //nolint:wrapcheck // Path gives the full error.
			}

			statusx.Done("Assets server stopped")

			return nil
		},
	}

	serveCmd.Flags().StringP("address", "a", "0.0.0.0", "Address to listen on")
	serveCmd.Flags().StringP("port", "p", "8080", "Port to listen on")
	serveCmd.Flags().StringP("path", "x", "./", "Path to serve assets from")

	return serveCmd
}
