// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package connect

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/furyctl/cmd"
	"github.com/sighupio/furyctl/internal/analytics"
	"github.com/sighupio/furyctl/internal/app"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

type OpenVPNCmdFlags struct {
	Profile     string
	FuryctlPath string
	Outdir      string
}

var (
	ErrParsingFlag         = errors.New("cannot parse command-line flag")
	ErrProfileFlagRequired = errors.New("profile flag is required")
	ErrRunningOpenVPN      = errors.New("cannot run openvpn")
	ErrCannotGetHomeDir    = errors.New("cannot get current working directory")
	cmdEvent               analytics.Event   //nolint:gochecknoglobals // needed for cobra/viper compatibility.
	openvpnCmd             = &cobra.Command{ //nolint:gochecknoglobals // needed for cobra/viper compatibility.
		Use:   "openvpn",
		Short: "Connect to OpenVPN with the specified profile name",
		PreRun: func(cmd *cobra.Command, _ []string) {
			cmdEvent = analytics.NewCommandEvent(cobrax.GetFullname(cmd))
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			ctn := app.GetContainerInstance()

			tracker := ctn.Tracker()
			tracker.Flush()

			logrus.Info("Connecting to OpenVPN...")

			// Parse flags.
			logrus.Debug("Parsing VPN Flags...")
			flags := getOpenVPNCmdFlags()

			if flags.Profile == "" {
				return ErrProfileFlagRequired
			}

			// Get home dir.
			logrus.Debug("Getting Home Directory Path...")
			outDir := flags.Outdir
			homeDir, err := os.UserHomeDir()
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("%w: %w", ErrCannotGetHomeDir, err)
			}

			if outDir == "" {
				outDir = homeDir
			}

			// Parse furyctl.yaml config.
			logrus.Debug("Parsing furyctl.yaml file...")
			furyctlConf, err := yamlx.FromFileV3[config.Furyctl](flags.FuryctlPath)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return err
			}

			// Set common paths.
			logrus.Debug("Setting common paths...")
			basePath := filepath.Join(outDir, ".furyctl", furyctlConf.Metadata.Name)
			openVPNWorkDir := filepath.Join(basePath, "infrastructure", "terraform", "secrets")

			executor := execx.NewStdExecutor()
			openVPNCmd := execx.NewCmd("sudo", execx.CmdOptions{
				Args:     []string{"openvpn", "--config", fmt.Sprintf("%s-%s.ovpn", furyctlConf.Metadata.Name, flags.Profile)},
				Executor: executor,
				WorkDir:  openVPNWorkDir,
			})

			// Start openvpn process.
			logrus.Debug("Running OpenVPN...")
			if err := openVPNCmd.Run(); err != nil {
				err = fmt.Errorf("%w: %w", ErrRunningOpenVPN, err)
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return err
			}

			return nil
		},
	}
)

//nolint:gochecknoinits // this pattern requires init function to work.
func init() {
	openvpnCmd.Flags().StringP(
		"config",
		"c",
		"furyctl.yaml",
		"Path to the configuration file",
	)

	openvpnCmd.Flags().StringP(
		"profile",
		"p",
		"",
		"Name of to the OpenVPN profile",
	)

	if err := viper.BindPFlags(openvpnCmd.Flags()); err != nil {
		logrus.Fatalf("error while binding flags: %v", err)
	}

	cmd.ConnectCmd.AddCommand(openvpnCmd)
}

func getOpenVPNCmdFlags() OpenVPNCmdFlags {
	return OpenVPNCmdFlags{
		Profile:     viper.GetString("profile"),
		FuryctlPath: viper.GetString("config"),
		Outdir:      viper.GetString("outdir"),
	}
}
