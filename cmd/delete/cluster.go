// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package del

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sighupio/fury-distribution/pkg/config"
	"github.com/sighupio/furyctl/internal/cluster"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

var (
	ErrYamlUnmarshalFile = errors.New("error unmarshaling yaml file")
	ErrDebugFlagNotSet   = errors.New("debug flag not set")
	ErrFuryctlFlagNotSet = errors.New("furyctl flag not set")
	ErrPhaseFlagNotSet   = errors.New("phase flag not set")
	ErrDryRunFlagNotSet  = errors.New("dry-run flag not set")
	ErrForceFlagNotSet   = errors.New("force flag not set")
)

func NewClusterCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "Deletes a cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			debug, ok := cobrax.Flag[bool](cmd, "debug").(bool)
			if !ok {
				return ErrDebugFlagNotSet
			}

			furyctlPath, ok := cobrax.Flag[string](cmd, "config").(string)
			if !ok {
				return ErrFuryctlFlagNotSet
			}

			phase, ok := cobrax.Flag[string](cmd, "phase").(string)
			if !ok {
				return ErrPhaseFlagNotSet
			}

			dryRun, ok := cobrax.Flag[bool](cmd, "dry-run").(bool)
			if !ok {
				return ErrDryRunFlagNotSet
			}

			force, ok := cobrax.Flag[bool](cmd, "force").(bool)
			if !ok {
				return ErrForceFlagNotSet
			}

			execx.Debug = debug

			// Init paths.
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("error while getting user home directory: %w", err)
			}

			minimalConf, err := yamlx.FromFileV3[config.Furyctl](furyctlPath)
			if err != nil {
				return fmt.Errorf("%w: %s", ErrYamlUnmarshalFile, err)
			}

			basePath := filepath.Join(homeDir, ".furyctl", minimalConf.Metadata.Name)

			clusterDeleter, err := cluster.NewDeleter(minimalConf, phase, basePath)
			if err != nil {
				return fmt.Errorf("error while initializing cluster deleter: %w", err)
			}

			if !force {
				_, err = fmt.Println("WARNING: You are about to delete a cluster. This action is irreversible.")
				if err != nil {
					return fmt.Errorf("error while printing to stdout: %w", err)
				}

				_, err = fmt.Println("Are you sure you want to continue? Only 'yes' will be accepted to confirm.")
				if err != nil {
					return fmt.Errorf("error while printing to stdout: %w", err)
				}

				if !askForConfirmation() {
					return nil
				}
			}

			err = clusterDeleter.Delete(dryRun)
			if err != nil {
				return fmt.Errorf("error while deleting cluster: %w", err)
			}

			logrus.Info("Cluster deleted successfully!")

			return nil
		},
	}

	cmd.Flags().StringP(
		"config",
		"c",
		"furyctl.yaml",
		"Path to the furyctl.yaml file",
	)

	cmd.Flags().StringP(
		"phase",
		"p",
		"",
		"Phase to execute",
	)

	cmd.Flags().Bool(
		"dry-run",
		false,
		"Allows to inspect what resources will be deleted",
	)

	cmd.Flags().Bool(
		"force",
		false,
		"Force deletion of the cluster",
	)

	return cmd
}

func askForConfirmation() bool {
	reader := bufio.NewReader(os.Stdin)

	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	response = strings.TrimSuffix(response, "\n")

	return strings.Compare(response, "yes") == 0
}
