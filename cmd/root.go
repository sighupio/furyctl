// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
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
	Log              string
}

type RootCommand struct {
	*cobra.Command
	config *rootConfig
}

const (
	timeout      = 100 * time.Millisecond
	spinnerStyle = 11
)

func NewRootCommand(versions map[string]string, logFile *os.File, tracker *analytics.Tracker) *RootCommand {
	// Update channels.
	r := make(chan app.Release, 1)
	e := make(chan error, 1)
	eventCh := make(chan analytics.Event, 1)

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
				var err error

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

				logPath, ok := cobrax.Flag[string](cmd, "log").(string)
				if ok && logPath != "stdout" {
					if logPath == "" {
						homeDir, err := os.UserHomeDir()
						if err != nil {
							logrus.Fatalf("error while getting user home directory: %v", err)
						}

						logPath = filepath.Join(homeDir, ".furyctl", "furyctl.log")
					}

					logFile, err = createLogFile(logPath)
					if err != nil {
						logrus.Fatalf("%v", err)
					}

					execx.LogFile = logFile
				}

				// Set log level.
				dflag, ok := cobrax.Flag[bool](cmd, "debug").(bool)
				if ok {
					logrusx.InitLog(logFile, dflag)
				}

				logrus.Debugf("logging to: %s", logPath)

				// Configure analytics.
				aflag, ok := cobrax.Flag[bool](cmd, "disable-analytics").(bool)
				if ok && aflag {
					tracker.Disable(aflag)
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
			PersistentPostRun: func(cmd *cobra.Command, _ []string) {
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
				// Track analytics events
				select {
				case event := <-eventCh:
					aflag, ok := cobrax.Flag[bool](cmd, "disable-analytics").(bool)
					if ok && aflag {
						if err := tracker.Track(event); err != nil {
							logrus.Debug(err)
						}
					}
				}
			},
		},
		config: cfg,
	}

	viper.AutomaticEnv()
	viper.SetEnvPrefix("furyctl")

	rootCmd.PersistentFlags().BoolVarP(&rootCmd.config.Debug, "debug", "D", false, "Enables furyctl debug output")
	rootCmd.PersistentFlags().BoolVarP(&rootCmd.config.DisableAnalytics, "disable-analytics", "d", false, "Disable analytics")
	rootCmd.PersistentFlags().BoolVarP(&rootCmd.config.DisableTty, "no-tty", "T", false, "Disable TTY")
	rootCmd.PersistentFlags().StringVarP(&rootCmd.config.Workdir, "workdir", "w", "", "Switch to a different working directory before executing the given subcommand.")
	rootCmd.PersistentFlags().StringVarP(&rootCmd.config.Log, "log", "l", "", "Path to the log file or stdout to log to standard output")

	rootCmd.AddCommand(NewCompletionCmd(eventCh))
	rootCmd.AddCommand(NewCreateCommand(versions["version"], eventCh))
	rootCmd.AddCommand(NewDownloadCmd(versions["version"], eventCh))
	rootCmd.AddCommand(NewDumpCmd(eventCh))
	rootCmd.AddCommand(NewValidateCommand(versions["version"], eventCh))
	rootCmd.AddCommand(NewVersionCmd(versions, eventCh))
	rootCmd.AddCommand(NewDeleteCommand(versions["version"])) // TODO: add tracking

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

func createLogFile(path string) (*os.File, error) {
	// Create the log directory if it doesn't exist.
	if err := os.MkdirAll(filepath.Dir(path), iox.UserGroupPerm); err != nil {
		return nil, fmt.Errorf("error while creating log file: %w", err)
	}

	logFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, iox.RWPermAccess)
	if err != nil {
		return nil, fmt.Errorf("error while creating log file: %w", err)
	}

	return logFile, nil
}
