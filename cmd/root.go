// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"time"

	"github.com/briandowns/spinner"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/sighupio/furyctl/internal/analytics"
	"github.com/sighupio/furyctl/internal/cobrax"
	"github.com/sighupio/furyctl/internal/io"
)

type rootConfig struct {
	Spinner          *spinner.Spinner
	Debug            bool
	DisableAnalytics bool
	DisableTty       bool
}

type RootCommand struct {
	*cobra.Command
	config *rootConfig
}

func NewRootCommand(versions map[string]string) *RootCommand {
	cfg := &rootConfig{}
	rootCmd := &RootCommand{
		Command: &cobra.Command{
			Use:   "furyctl",
			Short: "The multi-purpose command line tool for the Kubernetes Fury Distribution",
			Long: `The multi-purpose command line tool for the Kubernetes Fury Distribution.

Furyctl is a simple CLI tool to:

- download and manage the Kubernetes Fury Distribution (KFD) modules
- create and manage Kubernetes Fury clusters
`,
			SilenceUsage:  true,
			SilenceErrors: true,
			PersistentPreRun: func(cmd *cobra.Command, _ []string) {
				// Configure the spinner
				w := logrus.StandardLogger().Out
				if cobrax.Flag[bool](cmd, "no-tty").(bool) {
					w = io.NewNullWriter()
					f := new(logrus.TextFormatter)
					f.DisableColors = true
					logrus.SetFormatter(f)
				}
				cfg.Spinner = spinner.New(spinner.CharSets[11], 100*time.Millisecond, spinner.WithWriter(w))

				// Set log level
				if cobrax.Flag[bool](cmd, "debug").(bool) {
					logrus.SetLevel(logrus.DebugLevel)
				} else {
					logrus.SetLevel(logrus.InfoLevel)
				}

				// Configure analytics
				analytics.Version(versions["version"])
				analytics.Disable(cobrax.Flag[bool](cmd, "disable-analytics").(bool))
			},
		},
		config: cfg,
	}

	viper.AutomaticEnv()
	viper.SetEnvPrefix("furyctl")

	rootCmd.PersistentFlags().BoolVarP(&rootCmd.config.Debug, "debug", "D", false, "Enables furyctl debug output")
	rootCmd.PersistentFlags().BoolVarP(&rootCmd.config.DisableAnalytics, "disable", "d", false, "Disable analytics")
	rootCmd.PersistentFlags().BoolVarP(&rootCmd.config.DisableTty, "no-tty", "T", false, "Disable TTY")

	rootCmd.AddCommand(NewCompletionCmd())
	rootCmd.AddCommand(NewDownloadCmd(versions["version"]))
	rootCmd.AddCommand(NewDumpCmd())
	rootCmd.AddCommand(NewValidateCommand(versions["version"]))
	rootCmd.AddCommand(NewVersionCmd(versions))

	return rootCmd
}
