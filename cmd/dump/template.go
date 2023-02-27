// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dump

import (
	"errors"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/internal/analytics"
	"github.com/sighupio/furyctl/internal/cmd/cmdutil"
	"github.com/sighupio/furyctl/internal/config"
	"github.com/sighupio/furyctl/internal/distribution"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
	netx "github.com/sighupio/furyctl/internal/x/net"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

var ErrParsingFlag = errors.New("error while parsing flag")

type TemplateCmdFlags struct {
	DryRun         bool
	NoOverwrite    bool
	OutDir         string
	FuryctlPath    string
	DistroLocation string
}

func NewTemplateCmd(tracker *analytics.Tracker) *cobra.Command {
	var cmdEvent analytics.Event

	cmd := &cobra.Command{
		Use:   "template",
		Short: "Renders the distribution's manifests from a template and a configuration file",
		Long: `Generates a folder with the Kustomization project for deploying Kubernetes Fury Distribution into a cluster.
The generated folder will be created starting from a provided template and the parameters set in a configuration file that is merged with default values.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PreRun: func(cmd *cobra.Command, _ []string) {
			cmdEvent = analytics.NewCommandEvent(cobrax.GetFullname(cmd))
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Get flags.
			flags, err := getDumpTemplateCmdFlags(cmd, tracker, cmdEvent)
			if err != nil {
				return err
			}

			// Init collaborators.
			client := netx.NewGoGetterClient()
			distrodl := distribution.NewDownloader(client)

			// Download the distribution.
			logrus.Info("Downloading distribution...")
			res, err := distrodl.Download(flags.DistroLocation, flags.FuryctlPath)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error downloading distribution: %w", err)
			}

			// Validate the furyctl.yaml file.
			logrus.Info("Validating configuration file...")
			if err := config.Validate(flags.FuryctlPath, res.RepoPath); err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while validating configuration file: %w", err)
			}

			furyctlFile, err := yamlx.FromFileV2[map[any]any](flags.FuryctlPath)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("%s - %w", flags.FuryctlPath, err)
			}

			logrus.Info("Generating distribution manifests...")

			distroManBuilder := distribution.NewManifestBuilder(
				furyctlFile,
				res.RepoPath,
				flags.OutDir,
				flags.NoOverwrite,
				flags.DryRun,
			)

			if err := distroManBuilder.Build(); err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while generating distribution manifests: %w", err)
			}

			logrus.Info("Distribution manifests generated successfully")

			cmdEvent.AddSuccessMessage("Distribution manifests generated successfully")
			tracker.Track(cmdEvent)

			return nil
		},
	}

	cmd.Flags().Bool(
		"dry-run",
		false,
		"Furyctl will try its best to generate the manifests despite the errors",
	)

	cmd.Flags().Bool(
		"no-overwrite",
		false,
		"Stop if target directory is not empty",
	)

	cmd.Flags().StringP(
		"out-dir",
		"o",
		"distribution",
		"Location where to generate the distribution template. Defaults to distribution folder in the current "+
			"directory.",
	)

	cmd.Flags().StringP(
		"distro-location",
		"",
		"",
		"Location where to download schemas, defaults and the distribution manifest. "+
			"It can either be a local path(eg: /path/to/fury/distribution) or "+
			"a remote URL(eg: git::git@github.com:sighupio/fury-distribution?ref=BRANCH_NAME&depth=1). "+
			"Any format supported by hashicorp/go-getter can be used.",
	)

	cmd.Flags().StringP(
		"config",
		"c",
		"furyctl.yaml",
		"Path to the configuration file",
	)

	return cmd
}

func getDumpTemplateCmdFlags(cmd *cobra.Command, tracker *analytics.Tracker, cmdEvent analytics.Event) (TemplateCmdFlags, error) {
	dryRun, err := cmdutil.BoolFlag(cmd, "dry-run", tracker, cmdEvent)
	if err != nil {
		return TemplateCmdFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "dry-run")
	}

	noOverwrite, err := cmdutil.BoolFlag(cmd, "no-overwrite", tracker, cmdEvent)
	if err != nil {
		return TemplateCmdFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "no-overwrite")
	}

	outDir, err := cmdutil.StringFlag(cmd, "out-dir", tracker, cmdEvent)
	if err != nil {
		return TemplateCmdFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "out-dir")
	}

	distroLocation, err := cmdutil.StringFlag(cmd, "distro-location", tracker, cmdEvent)
	if err != nil {
		return TemplateCmdFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "distro-location")
	}

	furyctlPath, err := cmdutil.StringFlag(cmd, "config", tracker, cmdEvent)
	if err != nil {
		return TemplateCmdFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "config")
	}

	return TemplateCmdFlags{
		DryRun:         dryRun,
		NoOverwrite:    noOverwrite,
		OutDir:         outDir,
		DistroLocation: distroLocation,
		FuryctlPath:    furyctlPath,
	}, nil
}
