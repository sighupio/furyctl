// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package create

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	distroconf "github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/furyctl/internal/analytics"
	"github.com/sighupio/furyctl/internal/cmd/cmdutil"
	"github.com/sighupio/furyctl/internal/config"
	"github.com/sighupio/furyctl/internal/dependencies"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/semver"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	netx "github.com/sighupio/furyctl/internal/x/net"
)

var (
	ErrParsingFlag          = errors.New("error while parsing flag")
	ErrMandatoryFlag        = errors.New("flag must be specified")
	ErrConfigCreationFailed = fmt.Errorf("config creation failed")
)

func NewConfigCommand(tracker *analytics.Tracker) *cobra.Command {
	var cmdEvent analytics.Event

	cmd := &cobra.Command{
		Use:   "config",
		Short: "Scaffolds a new furyctl configuration file",
		PreRun: func(cmd *cobra.Command, _ []string) {
			cmdEvent = analytics.NewCommandEvent(cobrax.GetFullname(cmd))
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Get flags.
			debug, err := cmdutil.BoolFlag(cmd, "debug", tracker, cmdEvent)
			if err != nil {
				return fmt.Errorf("%w: %s", ErrParsingFlag, "debug")
			}

			furyctlPath, err := cmdutil.StringFlag(cmd, "config", tracker, cmdEvent)
			if err != nil {
				return fmt.Errorf("%w: config", ErrParsingFlag)
			}

			distroLocation, err := cmdutil.StringFlag(cmd, "distro-location", tracker, cmdEvent)
			if err != nil {
				return fmt.Errorf("%w: %s", ErrParsingFlag, "distro-location")
			}

			version, err := cmdutil.StringFlag(cmd, "version", tracker, cmdEvent)
			if err != nil {
				return fmt.Errorf("%w: version", ErrParsingFlag)
			}
			if version == "" {
				return fmt.Errorf("%w: version", ErrMandatoryFlag)
			}

			kind, err := cmdutil.StringFlag(cmd, "kind", tracker, cmdEvent)
			if err != nil {
				return fmt.Errorf("%w: kind", ErrParsingFlag)
			}
			if kind == "" {
				return fmt.Errorf("%w: kind", ErrMandatoryFlag)
			}

			apiVersion, err := cmdutil.StringFlag(cmd, "api-version", tracker, cmdEvent)
			if err != nil {
				return fmt.Errorf("%w: api-version", ErrParsingFlag)
			}

			name, err := cmdutil.StringFlag(cmd, "name", tracker, cmdEvent)
			if err != nil {
				return fmt.Errorf("%w: name", ErrParsingFlag)
			}

			https, err := cmdutil.BoolFlag(cmd, "https", tracker, cmdEvent)
			if err != nil {
				return fmt.Errorf("%w: https", ErrParsingFlag)
			}

			minimalConf := distroconf.Furyctl{
				APIVersion: apiVersion,
				Kind:       kind,
				Metadata: distroconf.FuryctlMeta{
					Name: name,
				},
				Spec: distroconf.FuryctlSpec{
					DistributionVersion: semver.EnsurePrefix(version),
				},
			}

			// Init collaborators.
			distrodl := distribution.NewDownloader(netx.NewGoGetterClient(), https)
			executor := execx.NewStdExecutor()
			depsvl := dependencies.NewValidator(executor, "", "", false)

			// Init packages.
			execx.Debug = debug

			// Validate path.
			if _, err := os.Stat(furyctlPath); err == nil {
				absPath, err := filepath.Abs(furyctlPath)
				if err != nil {
					return fmt.Errorf("%w: error while getting absolute path %v", ErrConfigCreationFailed, err)
				}

				p := filepath.Dir(absPath)

				return fmt.Errorf(
					"%w: a configuration file already exists in %s, please remove it and try again",
					ErrConfigCreationFailed,
					p,
				)
			}

			// Validate base requirements.
			if err := depsvl.ValidateBaseReqs(); err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while validating requirements: %w", err)
			}

			// Download the distribution.
			logrus.Info("Downloading distribution...")
			res, err := distrodl.DoDownload(distroLocation, minimalConf)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("failed to download distribution: %w", err)
			}

			cmdEvent.AddClusterDetails(analytics.ClusterDetails{
				Provider:   res.MinimalConf.Kind,
				KFDVersion: res.DistroManifest.Version,
			})

			data := map[string]string{
				"Kind":                kind,
				"Name":                name,
				"DistributionVersion": semver.EnsurePrefix(version),
			}

			out, err := config.Create(res, furyctlPath, cmdEvent, tracker, data)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("failed to create configuration file: %w", err)
			}

			logrus.Infof("Configuration file created successfully at: %s", out.Name())

			cmdEvent.AddSuccessMessage(fmt.Sprintf("Configuration file created successfully at: %s", out.Name()))
			tracker.Track(cmdEvent)

			return nil
		},
	}

	cmd.Flags().StringP(
		"config",
		"c",
		"furyctl.yaml",
		"Path to the configuration file",
	)

	cmd.Flags().StringP(
		"distro-location",
		"",
		"",
		"Base URL used to download schemas, defaults and the distribution manifest. "+
			"It can either be a local path(eg: /path/to/fury/distribution) or "+
			"a remote URL(eg: git::git@github.com:sighupio/fury-distribution?depth=1&ref=BRANCH_NAME)."+
			"Any format supported by hashicorp/go-getter can be used.",
	)

	cmd.Flags().StringP(
		"version",
		"v",
		"",
		"Kubernetes Fury Distribution version to use (eg: v1.24.1)",
	)

	cmd.Flags().StringP(
		"kind",
		"k",
		"",
		"Type of cluster to create (eg: EKSCluster, KFDDistribution, OnPremises)",
	)

	cmd.Flags().StringP(
		"api-version",
		"a",
		"kfd.sighup.io/v1alpha2",
		"Version of the API to use for the selected kind (eg: kfd.sighup.io/v1alpha2)",
	)

	cmd.Flags().StringP(
		"name",
		"n",
		"example",
		"Name of cluster to create",
	)

	return cmd
}
