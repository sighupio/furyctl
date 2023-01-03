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

	"github.com/sighupio/furyctl/internal/analytics"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/cmd/cmdutil"
	"github.com/sighupio/furyctl/internal/dependencies"
	"github.com/sighupio/furyctl/internal/distribution"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	netx "github.com/sighupio/furyctl/internal/x/net"
)

var (
	ErrParsingFlag   = errors.New("error while parsing flag")
	ErrKubeconfigReq = errors.New("$KUBECONFIG is not set, so --kubeconfig is required when doing distribution phase")
)

func NewClusterCmd(version string, tracker *analytics.Tracker) *cobra.Command {
	var cmdEvent analytics.Event

	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "Deletes a cluster",
		PreRun: func(cmd *cobra.Command, _ []string) {
			cmdEvent = analytics.NewCommandEvent(cobrax.GetFullname(cmd))
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			debug, err := cmdutil.BoolFlag(cmd, "debug", tracker, cmdEvent)
			if err != nil {
				return fmt.Errorf("%w: debug", ErrParsingFlag)
			}

			furyctlPath, err := cmdutil.StringFlag(cmd, "config", tracker, cmdEvent)
			if err != nil {
				return fmt.Errorf("%w: config", ErrParsingFlag)
			}

			distroLocation, err := cmdutil.StringFlag(cmd, "distro-location", tracker, cmdEvent)
			if err != nil {
				return fmt.Errorf("%w: distro-location", ErrParsingFlag)
			}

			phase, err := cmdutil.StringFlag(cmd, "phase", tracker, cmdEvent)
			if err != nil {
				return fmt.Errorf("%w: phase", ErrParsingFlag)
			}

			binPath := cobrax.Flag[string](cmd, "bin-path").(string) //nolint:errcheck,forcetypeassert // optional flag
			dryRun, err := cmdutil.BoolFlag(cmd, "dry-run", tracker, cmdEvent)
			if err != nil {
				return fmt.Errorf("%w: dry-run", ErrParsingFlag)
			}

			force, err := cmdutil.BoolFlag(cmd, "force", tracker, cmdEvent)
			if err != nil {
				return fmt.Errorf("%w: force", ErrParsingFlag)
			}

			kubeconfig, err := cmdutil.StringFlag(cmd, "kubeconfig", tracker, cmdEvent)
			if err != nil {
				return fmt.Errorf("%w: %s", ErrParsingFlag, "kubeconfig")
			}

			// Init paths.
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("error while getting user home directory: %w", err)
			}

			// Check if kubeconfig is needed.
			if phase == "distribution" || phase == "" {
				if kubeconfig == "" {
					kubeconfigFromEnv := os.Getenv("KUBECONFIG")

					if kubeconfigFromEnv == "" {
						return ErrKubeconfigReq
					}

					logrus.Warnf("Missing --kubeconfig flag, fallback to KUBECONFIG from environment: %s", kubeconfigFromEnv)
				}
			}

			if binPath == "" {
				binPath = filepath.Join(homeDir, ".furyctl", "bin")
			}

			// Init first half of collaborators.
			client := netx.NewGoGetterClient()
			executor := execx.NewStdExecutor()
			distrodl := distribution.NewDownloader(client)

			execx.Debug = debug

			// Download the distribution.
			logrus.Info("Downloading distribution...")
			res, err := distrodl.Download(version, distroLocation, furyctlPath)
			if err != nil {
				err = fmt.Errorf("error while downloading distribution: %w", err)

				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return err
			}

			basePath := filepath.Join(homeDir, ".furyctl", res.MinimalConf.Metadata.Name)

			// Init second half of collaborators.
			depsvl := dependencies.NewValidator(executor, binPath)

			// Validate the dependencies.
			logrus.Info("Validating dependencies...")
			if err := depsvl.Validate(res); err != nil {
				return fmt.Errorf("error while validating dependencies: %w", err)
			}

			clusterDeleter, err := cluster.NewDeleter(
				res.MinimalConf,
				res.DistroManifest,
				phase,
				basePath,
				binPath,
				kubeconfig,
			)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while initializing cluster deleter: %w", err)
			}

			if !force {
				_, err = fmt.Println("WARNING: You are about to delete a cluster. This action is irreversible.")
				if err != nil {
					cmdEvent.AddErrorMessage(err)
					tracker.Track(cmdEvent)

					return fmt.Errorf("error while printing to stdout: %w", err)
				}

				_, err = fmt.Println("Are you sure you want to continue? Only 'yes' will be accepted to confirm.")
				if err != nil {
					cmdEvent.AddErrorMessage(err)
					tracker.Track(cmdEvent)

					return fmt.Errorf("error while printing to stdout: %w", err)
				}

				if !askForConfirmation() {
					return nil
				}
			}

			err = clusterDeleter.Delete(dryRun)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while deleting cluster: %w", err)
			}

			logrus.Info("Cluster deleted successfully!")

			cmdEvent.AddSuccessMessage("Cluster deleted successfully!")
			tracker.Track(cmdEvent)

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
		"distro-location",
		"",
		"",
		"Base URL used to download schemas, defaults and the distribution manifest. "+
			"It can either be a local path(eg: /path/to/fury/distribution) or "+
			"a remote URL(eg: https://git@github.com/sighupio/fury-distribution?ref=BRANCH_NAME)."+
			"Any format supported by hashicorp/go-getter can be used.",
	)

	cmd.Flags().StringP(
		"bin-path",
		"b",
		"",
		"Path to the bin folder where all dependencies are installed",
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

	cmd.Flags().String(
		"kubeconfig",
		"",
		"Path to the kubeconfig file, mandatory if you want to run the distribution phase alone or "+
			"if you want to delete a cluster and $KUBECONFIG is not set",
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
