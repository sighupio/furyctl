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
	"github.com/sighupio/furyctl/internal/analytics"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

var (
	ErrParsingFlag         = errors.New("cannot parse command-line flag")
	ErrProfileFlagRequired = errors.New("profile flag is required")
	ErrRunningOpenVPN      = errors.New("cannot run openvpn")
	ErrCannotGetHomeDir    = errors.New("cannot get current working directory")
)

type OpenVPNCmdFlags struct {
	Profile     string
	FuryctlPath string
	Outdir      string
}

func NewOpenVPNCmd(tracker *analytics.Tracker) (*cobra.Command, error) {
	var cmdEvent analytics.Event

	cmd := &cobra.Command{
		Use:   "openvpn",
		Short: "Connect to OpenVPN with the specified profile name",
		PreRun: func(cmd *cobra.Command, _ []string) {
			cmdEvent = analytics.NewCommandEvent(cobrax.GetFullname(cmd))
		},
		RunE: func(_ *cobra.Command, _ []string) error {
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

	setupOpenVPNCmdFlags(cmd)

	if err := viper.BindPFlags(cmd.Flags()); err != nil {
		return nil, fmt.Errorf("error while binding flags: %w", err)
	}

	return cmd, nil
}

func getOpenVPNCmdFlags() OpenVPNCmdFlags {
	return OpenVPNCmdFlags{
		Profile:     viper.GetString("profile"),
		FuryctlPath: viper.GetString("config"),
		Outdir:      viper.GetString("outdir"),
	}
}

func setupOpenVPNCmdFlags(cmd *cobra.Command) {
	cmd.Flags().StringP(
		"config",
		"c",
		"furyctl.yaml",
		"Path to the configuration file",
	)

	cmd.Flags().StringP(
		"profile",
		"p",
		"",
		"Name of to the OpenVPN profile",
	)
}
