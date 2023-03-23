// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package connect

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/sighupio/fury-distribution/pkg/config"
	"github.com/sighupio/furyctl/internal/analytics"
	"github.com/sighupio/furyctl/internal/cmd/cmdutil"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

var (
	ErrParsingFlag = errors.New("error while parsing flag")
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
			// check if user is root
			// userIsRoot, err := osx.IsRoot()
			// if err != nil {
			// 	return fmt.Errorf("error while checking if user is root: %w", err)
			// }
			// if !userIsRoot {
			// 	return errors.New("please run this command as root (with 'sudo')")
			// }

			// parse flags
			flags, err := getOpenVPNCmdFlags(cmd, tracker, cmdEvent)
			if err != nil {
				return err
			}
			if flags.Profile == "" {
				return errors.New("profile flag is required")
			}

			// get home dir
			homeDir, err := os.UserHomeDir()
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)
				return fmt.Errorf("error while getting current working directory: %w", err)
			}

			// parse furyctl.yaml config
			furyctlConf, err := yamlx.FromFileV3[config.Furyctl](flags.FuryctlPath)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)
				return err
			}

			// set common paths
			basePath := filepath.Join(homeDir, ".furyctl", furyctlConf.Metadata.Name)
			openVPNWorkDir := filepath.Join(basePath, "infrastructure", "terraform", "secrets")
			// openVPNPidFile := filepath.Join(basePath, "openvpn.pid")

			// check if openvpn.pid file exist
			// _, err = os.Stat(openVPNPidFile)
			// if err == nil {
			// 	return errors.New("openvpn seems to be already connected, please use 'disconnect' command to disconnect and terminate the openvpn process")
			// }
			// if err != nil {
			// 	if !errors.Is(err, os.ErrNotExist) {
			// 		cmdEvent.AddErrorMessage(err)
			// 		tracker.Track(cmdEvent)
			// 		return err
			// 	}
			// }

			executor := execx.NewStdExecutor()
			openVPNCmd := execx.NewCmd("sudo", execx.CmdOptions{
				Args:     []string{"openvpn", "--config", fmt.Sprintf("%s-%s.ovpn", furyctlConf.Metadata.Name, flags.Profile)},
				Executor: executor,
				WorkDir:  openVPNWorkDir,
			})

			// start openvpn process
			if err := openVPNCmd.Run(); err != nil {
				err = fmt.Errorf("error while running openvpn: %w", err)
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)
				return err
			}

			// save process pid to openvpn.pid
			// if err := iox.WriteFile(openVPNPidFile, []byte(strconv.Itoa(openVPNCmd.Process.Pid))); err != nil {
			// 	return fmt.Errorf("error while writing pid file: %w", err)
			// }

			// cmdEvent.AddSuccessMessage("connect openvpn succeeded")
			// tracker.Track(cmdEvent)

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
