// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/sighupio/furyctl/internal/app"
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
	Outdir           string
	Log              string
}

type RootCommand struct {
	*cobra.Command
	config *rootConfig
}

const (
	timeout      = 100 * time.Millisecond
	logRndCeil   = 100000
	spinnerStyle = 11
)

func NewRootCmd() *RootCommand {
	rootCmd := &RootCommand{
		Command: &cobra.Command{
			Use:   "furyctl",
			Short: "The Swiss Army knife for the Kubernetes Fury Distribution",
			Long: `The multi-purpose command line tool for the Kubernetes Fury Distribution.

	furyctl is a command line interface tool to manage the full lifecycle of a Kubernetes Fury Cluster.
	`,
			SilenceUsage:  true,
			SilenceErrors: true,
			PersistentPreRun: func(cmd *cobra.Command, _ []string) {
				var err error
				var logFile *os.File
				defer logFile.Close()

				ctn := app.GetContainerInstance()

				tracker := ctn.Tracker()
				defer tracker.Flush()

				if cmd.Name() == "__complete" {
					oldPreRunFunc := cmd.PreRun

					cmd.PreRun = func(cmd *cobra.Command, args []string) {
						if oldPreRunFunc != nil {
							oldPreRunFunc(cmd, args)
						}

						logrus.SetLevel(logrus.FatalLevel)
					}
				}

				// Configure the spinner.
				cflag := viper.GetBool("no-tty")

				outDir := viper.GetString("outdir")

				homeDir, err := os.UserHomeDir()
				if err != nil {
					logrus.Fatalf("error while getting user home directory: %v", err)
				}

				if outDir == "" {
					outDir = homeDir
				}

				logPath := viper.GetString("log")
				if logPath != "stdout" {
					if logPath == "" {
						rndNum, err := rand.Int(rand.Reader, big.NewInt(logRndCeil))
						if err != nil {
							logrus.Fatalf("%v", err)
						}

						logPath = filepath.Join(
							outDir,
							".furyctl",
							fmt.Sprintf("furyctl.%d-%d.log", time.Now().Unix(), rndNum.Int64()),
						)
					}

					logFile, err = createLogFile(logPath)
					if err != nil {
						logrus.Fatalf("%v", err)
					}

					execx.LogFile = logFile
				}

				// Set log level.
				dflag := viper.GetBool("debug")
				logrusx.InitLog(logFile, dflag, cflag)

				logrus.Debugf("logging to: %s", logPath)

				// Configure analytics.
				aflag := viper.GetBool("disable-analytics")
				if aflag {
					tracker.Disable()
				}

				// Change working directory if it is specified.
				if workdir := viper.GetString("workdir"); workdir != "" {
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

				https := viper.GetBool("https")
				if !https {
					logrus.Warn("The --https flag is deprecated, if you want to use ssh protocol to download repositories use --git-protocol ssh")
				}
			},
		},
		config: &rootConfig{},
	}

	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().BoolVarP(
		&rootCmd.config.Debug,
		"debug",
		"D",
		false,
		"Enables furyctl debug output",
	)

	rootCmd.PersistentFlags().BoolVarP(
		&rootCmd.config.DisableAnalytics,
		"disable-analytics",
		"d",
		false,
		"Disable analytics",
	)

	rootCmd.PersistentFlags().BoolVarP(
		&rootCmd.config.DisableTty,
		"no-tty",
		"T",
		false,
		"Disable TTY making furyctl's output more friendly to non-interactive shells by disabling animations and colors",
	)

	rootCmd.PersistentFlags().StringVarP(
		&rootCmd.config.Workdir,
		"workdir",
		"w",
		"",
		"Switch to a different working directory before executing the given subcommand",
	)

	rootCmd.PersistentFlags().StringVarP(
		&rootCmd.config.Outdir,
		"outdir",
		"o",
		"",
		"Switch to a different working directory before executing the given subcommand",
	)

	rootCmd.PersistentFlags().StringVarP(
		&rootCmd.config.Log,
		"log",
		"l",
		"",
		"Path to the log file or set to 'stdout' to log to standard output (default: ~/.furyctl/furyctl.log)",
	)

	rootCmd.PersistentFlags().StringP(
		"git-protocol",
		"g",
		"https",
		"download repositories using the given protocol (options: https, ssh). Use when SSH traffic is being blocked or when SSH "+
			"client has not been configured\nset the GITHUB_TOKEN environment variable with your token to use "+
			"authentication while downloading, for example for private repositories",
	)

	rootCmd.PersistentFlags().BoolP(
		"https",
		"H",
		true,
		"DEPRECATED: by default furyctl uses https protocol to download repositories",
	)

	if err := viper.BindPFlags(rootCmd.PersistentFlags()); err != nil {
		logrus.Fatalf("error while binding flags: %v", err)
	}

	rootCmd.AddCommand(NewApplyCmd())
	rootCmd.AddCommand(NewCompletionCmd())
	rootCmd.AddCommand(NewConnectCmd())
	rootCmd.AddCommand(NewCreateCmd())
	rootCmd.AddCommand(NewDeleteCmd())
	rootCmd.AddCommand(NewDiffCmd())
	rootCmd.AddCommand(NewDownloadCmd())
	rootCmd.AddCommand(NewDumpCmd())
	rootCmd.AddCommand(NewGetCmd())
	rootCmd.AddCommand(NewLegacyCmd())
	rootCmd.AddCommand(NewValidateCmd())
	rootCmd.AddCommand(NewVersionCmd())
	rootCmd.AddCommand(NewRenewCmd())

	return rootCmd
}

func initConfig() {
	viper.SetEnvPrefix("FURYCTL")

	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	viper.AutomaticEnv()
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
