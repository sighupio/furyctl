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

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/furyctl/internal/analytics"
	"github.com/sighupio/furyctl/internal/cmd/cmdutil"
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
}

func NewOpenVPNCmd(tracker *analytics.Tracker) *cobra.Command {
	var cmdEvent analytics.Event

	cmd := &cobra.Command{
		Use:   "openvpn",
		Short: "Connect to OpenVPN with the specified profile name",
		PreRun: func(cmd *cobra.Command, _ []string) {
			cmdEvent = analytics.NewCommandEvent(cobrax.GetFullname(cmd))
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			logrus.Info("Connecting to OpenVPN...")

			// Parse flags.
			logrus.Debug("Parsing VPN Flags...")
			flags, err := getOpenVPNCmdFlags(cmd, tracker, cmdEvent)
			if err != nil {
				return err
			}

			if flags.Profile == "" {
				return ErrProfileFlagRequired
			}

			// Get home dir.
			logrus.Debug("Getting Home Directory Path...")
			homeDir, err := os.UserHomeDir()
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("%w: %w", ErrCannotGetHomeDir, err)
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
			basePath := filepath.Join(homeDir, ".furyctl", furyctlConf.Metadata.Name)
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

	return cmd
}

func getOpenVPNCmdFlags(cmd *cobra.Command, tracker *analytics.Tracker, cmdEvent analytics.Event) (OpenVPNCmdFlags, error) {
	furyctlPath, err := cmdutil.StringFlag(cmd, "config", tracker, cmdEvent)
	if err != nil {
		return OpenVPNCmdFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "config")
	}

	profile, err := cmdutil.StringFlag(cmd, "profile", tracker, cmdEvent)
	if err != nil {
		return OpenVPNCmdFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "profile")
	}

	return OpenVPNCmdFlags{
		Profile:     profile,
		FuryctlPath: furyctlPath,
	}, nil
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
