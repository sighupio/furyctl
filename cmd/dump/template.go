// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dump

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/sighupio/furyctl/internal/analytics"
	"github.com/sighupio/furyctl/internal/app"
	"github.com/sighupio/furyctl/internal/config"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/flags"
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
	ErrTargetIsNotEmpty = errors.New("directory is not empty, set --no-overwrite=false to overwrite it")
)

func NewTemplateCmd() *cobra.Command {
	var cmdEvent analytics.Event

	templateCmd := &cobra.Command{
		Use:   "template",
		Short: "Renders the distribution's code from template files parametrized with the configuration file",
		Long: `Generates a folder with the parametrized version of the Terraform and Kustomization code for deploying the SIGHUP Distribution into a cluster.
The command will dump into a 'distribution' folder in the working directory all the rendered files using the parameters set in the configuration file.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PreRun: func(cmd *cobra.Command, _ []string) {
			cmdEvent = analytics.NewCommandEvent(cobrax.GetFullname(cmd))

			// Load and validate flags from configuration FIRST.
			if err := flags.LoadAndMergeCommandFlags("dump"); err != nil {
				logrus.Fatalf("failed to load flags from configuration: %v", err)
			}

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
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return err
			}

			var distrodl *dist.Downloader

			client := netx.NewGoGetterClient()
			executor := execx.NewStdExecutor()
			depsvl := dependencies.NewValidator(executor, "", flags.FuryctlPath, false)

			if flags.DistroLocation == "" {
				distrodl = dist.NewCachingDownloader(client, flags.Outdir, flags.GitProtocol, flags.DistroPatchesLocation)
			} else {
				distrodl = dist.NewDownloader(client, flags.GitProtocol, flags.DistroPatchesLocation)
			}

			if err := depsvl.ValidateBaseReqs(); err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while validating requirements: %w", err)
			}

			logrus.Info("Downloading distribution...")
			res, err := distrodl.Download(flags.DistroLocation, flags.FuryctlPath)
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
				if err := config.Validate(flags.FuryctlPath, res.RepoPath); err != nil {
					cmdEvent.AddErrorMessage(err)
					tracker.Track(cmdEvent)

					return fmt.Errorf("error while validating configuration file: %w", err)
				}
			} else {
				logrus.Info("Skipping configuration file validation")
			}

			furyctlFile, err := yamlx.FromFileV2[map[any]any](flags.FuryctlPath)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("%s - %w", flags.FuryctlPath, err)
			}

			// Note: this is already the right working directory because it is updated in the root command.
			dumpDir := filepath.Join(viper.GetString("workdir"), "distribution")

			logrus.Info("Rendering distribution manifests...")

			distroManBuilder, err := distribution.NewIACBuilder(
				furyctlFile,
				res.RepoPath,
				res.MinimalConf.Kind,
				dumpDir,
				flags.FuryctlPath,
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
					return fmt.Errorf("%w: \"%s\"", ErrTargetIsNotEmpty, dumpDir)
				}

				return fmt.Errorf("error while generating distribution manifests: %w", err)
			}

			logrus.Info("Distribution manifests successfully dumped to ", dumpDir)

			cmdEvent.AddSuccessMessage("Distribution manifests dumped successfully")
			tracker.Track(cmdEvent)

			return nil
		},
	}

	templateCmd.Flags().Bool(
		"dry-run",
		false,
		"furyctl will try its best to generate the manifests despite the errors",
	)

	templateCmd.Flags().Bool(
		"no-overwrite",
		true,
		"Stop if target directory is not empty. WARNING: setting this to false will delete the folder and its content",
	)

	templateCmd.Flags().StringP(
		"distro-location",
		"",
		"",
		"Location where to download schemas, defaults and the distribution manifest from. "+
			"It can either be a local path(eg: /path/to/distribution) or "+
			"a remote URL(eg: git::git@github.com:sighupio/distribution?ref=BRANCH_NAME&depth=1). "+
			"Any format supported by hashicorp/go-getter can be used",
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
		"Location where the distribution's user-made patches can be downloaded from. "+
			"This can be either a local path (eg: /path/to/distro-patches) or "+
			"a remote URL (eg: git::git@github.com:your-org/distro-patches?depth=1&ref=BRANCH_NAME). "+
			"Any format supported by hashicorp/go-getter can be used."+
			" Patches within this location must be in a folder named after the distribution version (eg: v1.29.0) and "+
			"must have the same structure as the distribution's repository",
	)

	return templateCmd
}

func getDumpTemplateCmdFlags() (TemplateCmdFlags, error) {
	gitProtocol := viper.GetString("git-protocol")

	typedGitProtocol, err := git.NewProtocol(gitProtocol)
	if err != nil {
		return TemplateCmdFlags{}, fmt.Errorf("%w: %w", ErrParsingFlag, err)
	}

	furyctlPath := viper.GetString("config")

	if furyctlPath == "" {
		return TemplateCmdFlags{}, fmt.Errorf("%w --config: cannot be an empty string", ErrParsingFlag)
	}

	furyctlPath, err = filepath.Abs(furyctlPath)
	if err != nil {
		return TemplateCmdFlags{}, fmt.Errorf("error while getting configuration file absolute path: %w", err)
	}

	distroPatchesLocation := viper.GetString("distro-patches")
	if distroPatchesLocation != "" {
		distroPatchesLocation, err = filepath.Abs(distroPatchesLocation)
		if err != nil {
			return TemplateCmdFlags{}, fmt.Errorf("error while getting absolute path of distro patches location: %w", err)
		}
	}

	return TemplateCmdFlags{
		DryRun:                viper.GetBool("dry-run"),
		NoOverwrite:           viper.GetBool("no-overwrite"),
		SkipValidation:        viper.GetBool("skip-validation"),
		Outdir:                viper.GetString("outdir"),
		DistroLocation:        viper.GetString("distro-location"),
		FuryctlPath:           furyctlPath,
		GitProtocol:           typedGitProtocol,
		DistroPatchesLocation: distroPatchesLocation,
	}, nil
}
