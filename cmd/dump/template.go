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
	"github.com/spf13/viper"

	"github.com/sighupio/furyctl/internal/analytics"
	"github.com/sighupio/furyctl/internal/app"
	"github.com/sighupio/furyctl/internal/config"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/git"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	"github.com/sighupio/furyctl/pkg/dependencies"
	dist "github.com/sighupio/furyctl/pkg/distribution"
	"github.com/sighupio/furyctl/pkg/template"
	netx "github.com/sighupio/furyctl/pkg/x/net"
	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
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

var (
	ErrParsingFlag      = errors.New("error while parsing flag")
	ErrTargetIsNotEmpty = errors.New("output directory is not empty, set --no-overwrite=false to overwrite it")
)

func NewTemplateCmd() *cobra.Command {
	var cmdEvent analytics.Event

	templateCmd := &cobra.Command{
		Use:   "template",
		Short: "Renders the distribution's infrastructure code from template files and a configuration file",
		Long: `Generates a folder with the Terraform and Kustomization code for deploying the Kubernetes Fury Distribution into a cluster.
The generated folder will be created starting from a provided templates folder and the parameters set in a configuration file that is merged with default values.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PreRun: func(cmd *cobra.Command, _ []string) {
			cmdEvent = analytics.NewCommandEvent(cobrax.GetFullname(cmd))

			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				logrus.Fatalf("error while binding flags: %v", err)
			}
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			ctn := app.GetContainerInstance()

			tracker := ctn.Tracker()
			tracker.Flush()

			flags, err := getDumpTemplateCmdFlags()
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

			var distrodl *dist.Downloader

			client := netx.NewGoGetterClient()
			executor := execx.NewStdExecutor()
			depsvl := dependencies.NewValidator(executor, "", absFuryctlPath, false)

			if flags.DistroLocation == "" {
				distrodl = dist.NewCachingDownloader(client, outDir, flags.GitProtocol, absDistroPatchesLocation)
			} else {
				distrodl = dist.NewDownloader(client, flags.GitProtocol, absDistroPatchesLocation)
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

	templateCmd.Flags().Bool(
		"dry-run",
		false,
		"Furyctl will try its best to generate the manifests despite the errors",
	)

	templateCmd.Flags().Bool(
		"no-overwrite",
		true,
		"Stop if target directory is not empty",
	)

	templateCmd.Flags().StringP(
		"distro-location",
		"",
		"",
		"Location where to download schemas, defaults and the distribution manifest. "+
			"It can either be a local path(eg: /path/to/fury/distribution) or "+
			"a remote URL(eg: git::git@github.com:sighupio/fury-distribution?ref=BRANCH_NAME&depth=1). "+
			"Any format supported by hashicorp/go-getter can be used.",
	)

	templateCmd.Flags().StringP(
		"config",
		"c",
		"furyctl.yaml",
		"Path to the configuration file",
	)

	templateCmd.Flags().Bool(
		"skip-validation",
		false,
		"Skip validation of the configuration file",
	)

	templateCmd.Flags().String(
		"distro-patches",
		"",
		"Location where to download distribution's user-made patches from. "+
			"Any format supported by hashicorp/go-getter can be used.",
	)

	return templateCmd
}

func getDumpTemplateCmdFlags() (TemplateCmdFlags, error) {
	gitProtocol := viper.GetString("git-protocol")

	typedGitProtocol, err := git.NewProtocol(gitProtocol)
	if err != nil {
		return TemplateCmdFlags{}, fmt.Errorf("%w: %w", ErrParsingFlag, err)
	}

	return TemplateCmdFlags{
		DryRun:                viper.GetBool("dry-run"),
		NoOverwrite:           viper.GetBool("no-overwrite"),
		SkipValidation:        viper.GetBool("skip-validation"),
		Outdir:                viper.GetString("outdir"),
		DistroLocation:        viper.GetString("distro-location"),
		FuryctlPath:           viper.GetString("config"),
		GitProtocol:           typedGitProtocol,
		DistroPatchesLocation: viper.GetString("distro-patches"),
	}, nil
}
