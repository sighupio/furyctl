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

	"github.com/sighupio/furyctl/internal/io"
	"github.com/sighupio/furyctl/pkg/analytics"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"

	s                *spinner.Spinner
	debug            bool
	disableAnalytics bool
	disableTty       bool
)

// Execute is the main entrypoint of furyctl
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(bootstrapCmd)
	rootCmd.AddCommand(clusterCmd)
	rootCmd.AddCommand(completionCmd)
	rootCmd.AddCommand(templateCmd)
	rootCmd.AddCommand(validateCmd)
	rootCmd.AddCommand(vendorCmd)
	rootCmd.AddCommand(versionCmd)

	rootCmd.PersistentFlags().Bool("debug", false, "Enables furyctl debug output")
	rootCmd.PersistentFlags().BoolVarP(&disableAnalytics, "disable", "d", false, "Disable analytics")
	rootCmd.PersistentFlags().BoolVarP(&disableTty, "no-tty", "T", false, "Disable TTY")

	cobra.OnInitialize(func() {
		analytics.Version(version)
		analytics.Disable(disableAnalytics)

		w := logrus.StandardLogger().Out
		if disableTty {
			w = io.NewNullWriter()
			f := new(logrus.TextFormatter)
			f.DisableColors = true
			logrus.SetFormatter(f)
		}

		s = spinner.New(spinner.CharSets[11], 100*time.Millisecond, spinner.WithWriter(w))
	})

	viper.AutomaticEnv()
	viper.SetEnvPrefix("furyctl")
}

func bootstrapLogrus(cmd *cobra.Command) {
	d, err := cmd.Flags().GetBool("debug")
	if err != nil {
		logrus.Fatal(err)
	}

	if d {
		logrus.SetLevel(logrus.DebugLevel)
		debug = true

		return
	}

	logrus.SetLevel(logrus.InfoLevel)
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "furyctl",
	Short: "The multi-purpose command line tool for the Kubernetes Fury Distribution",
	Long: `The multi-purpose command line tool for the Kubernetes Fury Distribution.

Furyctl is a simple CLI tool to:

- download and manage the Kubernetes Fury Distribution (KFD) modules
- create and manage Kubernetes Fury clusters
`,
	SilenceUsage: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		bootstrapLogrus(cmd)
	},
}
