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
	"github.com/sighupio/furyctl/internal/flags"
	"github.com/sighupio/furyctl/internal/git"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
	logrusx "github.com/sighupio/furyctl/internal/x/logrus"
)

type rootConfig struct {
	Debug            bool
	DisableAnalytics bool
	DisableTty       bool
	GitProtocol      git.Protocol
	Log              string
	Outdir           string
	Spinner          *spinner.Spinner
	Workdir          string
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
			Short: "The Swiss Army knife for the SIGHUP Distribution",
			Long: `The multi-purpose command line tool for the SIGHUP Distribution.

furyctl is a command line interface tool to manage the full lifecycle of SIGHUP Distribution Clusters.`,
			SilenceUsage:  true,
			SilenceErrors: true,
			PersistentPreRun: func(cmd *cobra.Command, _ []string) {
				var err error
				var logFile *os.File
				defer logFile.Close()

				ctn := app.GetContainerInstance()

				// Configure analytics.
				tracker := ctn.Tracker()
				if viper.GetBool("disable-analytics") {
					tracker.Disable()
				}
				defer tracker.Flush()

				// Tab-autocompletion.
				if cmd.Name() == "__complete" {
					oldPreRunFunc := cmd.PreRun

					cmd.PreRun = func(cmd *cobra.Command, args []string) {
						if oldPreRunFunc != nil {
							oldPreRunFunc(cmd, args)
						}

						logrus.SetLevel(logrus.FatalLevel)
					}
				}

				// Load global flags from configuration file if available
				// This needs to happen early to allow global flags to affect subsequent operations
				flagsManager := flags.NewManager(".")
				if err := flagsManager.TryLoadFromCurrentDirectory("global"); err != nil {
					// Continue execution - global flags loading is optional
					logrus.Debugf("Failed to load global flags from current directory: %v", err)
				}

				// Change working directory (if it is specified) as first thing so all the following paths are relative
				// to this new working directory.
				if workdir := viper.GetString("workdir"); workdir != "" {
					// Get absolute path of workdir.
					absWorkdir, err := filepath.Abs(workdir)
					if err != nil {
						logrus.Fatalf("Error getting absolute path of workdir: %v", err)
					}

					if err := os.Chdir(absWorkdir); err != nil {
						logrus.Fatalf("Could not change directory: %v", err)
					}

					// We need to defer this log because we don't have logging configured yet.
					defer logrus.Debugf("Changed working directory to %s", absWorkdir)
					// Update the workdir in viper to the absolute path in case it is accessed from somewhere else.
					viper.Set("workdir", absWorkdir)
				}

				// Calculate the right outDir.
				outDir := viper.GetString("outdir")
				if outDir == "" {
					homeDir, err := os.UserHomeDir()
					if err != nil {
						logrus.Fatalf("error while getting user's home directory: %v", err)
					}

					outDir = homeDir
				}
				// The outdir path must be an absolute path, relative paths can be messy because the current directory
				// can change during execution, for example when rendering the templates.
				outDir, err = filepath.Abs(outDir)
				if err != nil {
					logrus.Fatalf("error while getting absolute path for outdir: %v", err)
				}
				// We need to defer this log because we don't have logging configured yet.
				defer logrus.Debugf("Outdir is set to %s", outDir)
				viper.Set("outdir", outDir)

				// Configure the logging options. Set the logging target (stdout or a file).
				// Note that we can't move this upper because we depend on calculating the workdir and outdir first.
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

				// Configure logging level and format.
				cflag := viper.GetBool("no-tty")
				dflag := viper.GetBool("debug")
				logrusx.InitLog(logFile, dflag, cflag)

				logrus.Debugf("Writing logs to %s", logPath)

				// Deprected flags.
				https := viper.GetBool("https")
				if !https {
					logrus.Warn("The --https flag is deprecated and https is the default, if you want to use SSH protocol to download repositories use --git-protocol ssh")
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
		"Enables furyctl debug output. This will greatly increase the verbosity. Debug logs and additional logs are also always written to the log file. See the --log flag.",
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
		"Switch to a different working directory before executing the given subcommand. NOTE: this will affect all the paths passed, including other flags like outdir and log, for example",
	)

	rootCmd.PersistentFlags().StringVarP(
		&rootCmd.config.Outdir,
		"outdir",
		"o",
		"",
		"Path where to create the \".furyctl\" data directory. Default is the user's home. Path is relative to --workdir",
	)

	rootCmd.PersistentFlags().StringVarP(
		&rootCmd.config.Log,
		"log",
		"l",
		"",
		"Path to a file or folder where to write logs to. Set to 'stdout' write to standard output. Target path will be created if it does not exists. Path is relative to --workdir. Default is '<outdir>/.furyctl/furyctl.<timestamp>-<random number>.log'",
	)

	rootCmd.PersistentFlags().VarP(
		&git.ProtocolFlag{Protocol: git.ProtocolHTTPS},
		"git-protocol",
		"g",
		"Download repositories using the given protocol. Use when SSH traffic is being blocked or when SSH "+
			"client has not been configured\nset the GITHUB_TOKEN environment variable with your "+
			"token to use authentication while downloading, for example for private repositories. "+
			"\nOptions are: "+strings.Join(git.ProtocolsS(), ", ")+"",
	)

	if err := rootCmd.RegisterFlagCompletionFunc("git-protocol", git.ProtocolFlagCompletion); err != nil {
		logrus.Fatalf("error while registering flag completion: %v", err)
	}

	rootCmd.PersistentFlags().BoolP(
		"https",
		"H",
		true,
		"DEPRECATED: by default furyctl uses https protocol to download repositories. See --git-protocol flag",
	)

	if err := viper.BindPFlags(rootCmd.PersistentFlags()); err != nil {
		logrus.Fatalf("error while binding flags: %v", err)
	}

	rootCmd.AddCommand(NewApplyCmd())
	rootCmd.AddCommand(NewCompletionCmd(rootCmd.Root()))
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
