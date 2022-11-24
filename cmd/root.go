// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"os"
	"path/filepath"
	"time"

	"github.com/briandowns/spinner"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/sighupio/furyctl/internal/analytics"
	"github.com/sighupio/furyctl/internal/app"
	"github.com/sighupio/furyctl/internal/semver"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
	logrusx "github.com/sighupio/furyctl/internal/x/logrus"
)

type rootConfig struct {
	Spinner          *spinner.Spinner
	Debug            bool
	DisableAnalytics bool
	DisableTty       bool
	Workdir          string
}

type RootCommand struct {
	*cobra.Command
	config *rootConfig
}

const (
	timeout      = 100 * time.Millisecond
	spinnerStyle = 11
)

func NewRootCommand(versions map[string]string, logFile *os.File) *RootCommand {
	// Update channels.
	r := make(chan app.Release, 1)
	e := make(chan error, 1)

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
				// Async check for updates.
				go checkUpdates(versions["version"], r, e)
				// Configure the spinner.
				w := logrus.StandardLogger().Out

				cflag, ok := cobrax.Flag[bool](cmd, "no-tty").(bool)
				if ok && cflag {
					w = iox.NewNullWriter()
					f := new(logrus.TextFormatter)
					f.DisableColors = true
					logrus.SetFormatter(f)
				}

				cfg.Spinner = spinner.New(spinner.CharSets[spinnerStyle], timeout, spinner.WithWriter(w))

				// Set log level.
				dflag, ok := cobrax.Flag[bool](cmd, "debug").(bool)
				if ok {
					logrusx.InitLog(logFile, dflag)
				}

				execx.LogFile = logFile

				// Configure analytics.
				a := analytics.New(true, versions["version"])
				aflag, ok := cobrax.Flag[bool](cmd, "disable-analytics").(bool)
				if ok && aflag {
					a.Disable(aflag)
				}
				// Change working directory if it is specified.
				if workdir, ok := cobrax.Flag[string](cmd, "workdir").(string); workdir != "" && ok {
					// Get absolute path of workdir.
					absWorkdir, err := filepath.Abs(workdir)
					if err != nil {
						logrus.Fatalf("Error getting absolute path of workdir: %v", err)
					}

					if err := os.Chdir(absWorkdir); err != nil {
						logrus.Fatalf("Could not change directory: %v", err)
					}

					logrus.Debugf("Changed working directory to %s", absWorkdir)
				}
			},
			PersistentPostRun: func(_ *cobra.Command, _ []string) {
				// Show update message if available at the end of the command.
				select {
				case release := <-r:
					if shouldUpgrade(release.Version, versions["version"]) {
						logrus.Infof("New furyctl version available: %s => %s", versions["version"], release.Version)
					}
				case err := <-e:
					if err != nil {
						logrus.Debugf("Error checking for updates: %s", err)
					}
				}
			},
		},
		config: cfg,
	}

	viper.AutomaticEnv()
	viper.SetEnvPrefix("furyctl")

	rootCmd.PersistentFlags().BoolVarP(&rootCmd.config.Debug, "debug", "D", false, "Enables furyctl debug output")
	rootCmd.PersistentFlags().BoolVarP(&rootCmd.config.DisableAnalytics, "disable", "d", false, "Disable analytics")
	rootCmd.PersistentFlags().BoolVarP(&rootCmd.config.DisableTty, "no-tty", "T", false, "Disable TTY")
	rootCmd.PersistentFlags().StringVarP(&rootCmd.config.Workdir, "workdir", "w", "", "Switch to a different working directory before executing the given subcommand.")

	rootCmd.AddCommand(NewCompletionCmd())
	rootCmd.AddCommand(NewCreateCommand(versions["version"]))
	rootCmd.AddCommand(NewDownloadCmd(versions["version"]))
	rootCmd.AddCommand(NewDumpCmd())
	rootCmd.AddCommand(NewValidateCommand(versions["version"]))
	rootCmd.AddCommand(NewVersionCmd(versions))
	rootCmd.AddCommand(NewDeleteCommand())

	return rootCmd
}

func shouldUpgrade(releaseVersion, currentVersion string) bool {
	if releaseVersion == "unknown" {
		return false
	}

	return semver.Gt(releaseVersion, currentVersion)
}

func checkUpdates(version string, rc chan app.Release, e chan error) {
	defer close(rc)
	defer close(e)

	if version == "unknown" {
		rc <- app.Release{Version: version}

		return
	}

	r, err := app.GetLatestRelease()
	if err != nil {
		e <- err

		return
	}

	rc <- r
}
