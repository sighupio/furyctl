// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dump

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/internal/analytics"
	"github.com/sighupio/furyctl/internal/cmd/cmdutil"
	"github.com/sighupio/furyctl/internal/config"
	"github.com/sighupio/furyctl/internal/dependencies"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/git"
	"github.com/sighupio/furyctl/internal/template"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	netx "github.com/sighupio/furyctl/internal/x/net"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

var (
	ErrParsingFlag      = errors.New("error while parsing flag")
	ErrTargetIsNotEmpty = errors.New("output directory is not empty, set --no-overwrite=false to overwrite it")
)

type TemplateCmdFlags struct {
	DryRun                bool
	NoOverwrite           bool
	SkipValidation        bool
	GitProtocol           git.Protocol
	Outdir                string
	FuryctlPath           string
	DistroLocation        string
	DistroPatchesLocation string
}

func NewTemplateCmd(tracker *analytics.Tracker) *cobra.Command {
	var cmdEvent analytics.Event

	cmd := &cobra.Command{
		Use:   "template",
		Short: "Renders the distribution's infrastructure code from template files and a configuration file",
		Long: `Generates a folder with the Terraform and Kustomization code for deploying the Kubernetes Fury Distribution into a cluster.
The generated folder will be created starting from a provided templates folder and the parameters set in a configuration file that is merged with default values.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PreRun: func(cmd *cobra.Command, _ []string) {
			cmdEvent = analytics.NewCommandEvent(cobrax.GetFullname(cmd))
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			flags, err := getDumpTemplateCmdFlags(cmd, tracker, cmdEvent)
			if err != nil {
				return err
			}

			absFuryctlPath, err := filepath.Abs(flags.FuryctlPath)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error: %w", err)
			}

			outDir := flags.Outdir

			// Get home dir.
			logrus.Debug("Getting Home Directory Path...")

			homeDir, err := os.UserHomeDir()
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while getting user home directory: %w", err)
			}

			if outDir == "" {
				outDir = homeDir
			}

			absDistroPatchesLocation := flags.DistroPatchesLocation

			if absDistroPatchesLocation != "" {
				absDistroPatchesLocation, err = filepath.Abs(flags.DistroPatchesLocation)
				if err != nil {
					cmdEvent.AddErrorMessage(err)
					tracker.Track(cmdEvent)

					return fmt.Errorf("error while getting absolute path of distro patches location: %w", err)
				}
			}

			var distrodl *distribution.Downloader

			client := netx.NewGoGetterClient()
			executor := execx.NewStdExecutor()
			depsvl := dependencies.NewValidator(executor, "", absFuryctlPath, false)

			if flags.DistroLocation == "" {
				distrodl = distribution.NewCachingDownloader(client, outDir, flags.GitProtocol, absDistroPatchesLocation)
			} else {
				distrodl = distribution.NewDownloader(client, flags.GitProtocol, absDistroPatchesLocation)
			}

			if err := depsvl.ValidateBaseReqs(); err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while validating requirements: %w", err)
			}

			logrus.Info("Downloading distribution...")
			res, err := distrodl.Download(flags.DistroLocation, absFuryctlPath)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error downloading distribution: %w", err)
			}

			cmdEvent.AddClusterDetails(analytics.ClusterDetails{
				Provider:   res.MinimalConf.Kind,
				KFDVersion: res.DistroManifest.Version,
				DryRun:     flags.DryRun,
			})

			if !flags.SkipValidation {
				logrus.Info("Validating configuration file...")
				if err := config.Validate(absFuryctlPath, res.RepoPath); err != nil {
					cmdEvent.AddErrorMessage(err)
					tracker.Track(cmdEvent)

					return fmt.Errorf("error while validating configuration file: %w", err)
				}
			}

			furyctlFile, err := yamlx.FromFileV2[map[any]any](absFuryctlPath)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("%s - %w", absFuryctlPath, err)
			}

			outDir = flags.Outdir

			currentDir, err := os.Getwd()
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while getting current directory: %w", err)
			}

			if outDir == "" {
				outDir = currentDir
			}

			outDir = filepath.Join(outDir, "distribution")

			logrus.Info("Generating distribution manifests...")

			distroManBuilder, err := distribution.NewIACBuilder(
				furyctlFile,
				res.RepoPath,
				res.MinimalConf.Kind,
				outDir,
				absFuryctlPath,
				flags.NoOverwrite,
				flags.DryRun,
			)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while creating distribution manifest builder: %w", err)
			}

			if err := distroManBuilder.Build(); err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				if errors.Is(err, template.ErrTargetIsNotEmpty) {
					return ErrTargetIsNotEmpty
				}

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
		true,
		"Stop if target directory is not empty",
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

	cmd.Flags().Bool(
		"skip-validation",
		false,
		"Skip validation of the configuration file",
	)

	cmd.Flags().String(
		"distro-patches",
		"",
		"Location where to download distribution's user-made patches from. "+
			cmdutil.AnyGoGetterFormatStr,
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

	skipValidation, err := cmdutil.BoolFlag(cmd, "skip-validation", tracker, cmdEvent)
	if err != nil {
		return TemplateCmdFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "skip-validation")
	}

	outdir, err := cmdutil.StringFlag(cmd, "outdir", tracker, cmdEvent)
	if err != nil {
		return TemplateCmdFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "outdir")
	}

	distroLocation, err := cmdutil.StringFlag(cmd, "distro-location", tracker, cmdEvent)
	if err != nil {
		return TemplateCmdFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "distro-location")
	}

	furyctlPath, err := cmdutil.StringFlag(cmd, "config", tracker, cmdEvent)
	if err != nil {
		return TemplateCmdFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "config")
	}

	gitProtocol, err := cmdutil.StringFlag(cmd, "git-protocol", tracker, cmdEvent)
	if err != nil {
		return TemplateCmdFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "git-protocol")
	}

	typedGitProtocol, err := git.NewProtocol(gitProtocol)
	if err != nil {
		return TemplateCmdFlags{}, fmt.Errorf("%w: %w", ErrParsingFlag, err)
	}

	distroPatchesLocation, err := cmdutil.StringFlag(cmd, "distro-patches", tracker, cmdEvent)
	if err != nil {
		return TemplateCmdFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "distro-patches")
	}

	return TemplateCmdFlags{
		DryRun:                dryRun,
		NoOverwrite:           noOverwrite,
		SkipValidation:        skipValidation,
		Outdir:                outdir,
		DistroLocation:        distroLocation,
		FuryctlPath:           furyctlPath,
		GitProtocol:           typedGitProtocol,
		DistroPatchesLocation: distroPatchesLocation,
	}, nil
}
