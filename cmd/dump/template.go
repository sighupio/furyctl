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
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/merge"
	"github.com/sighupio/furyctl/internal/template"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
	netx "github.com/sighupio/furyctl/internal/x/net"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

const (
	source           = "templates/distribution"
	suffix           = ".tpl"
	defaultsFileName = "furyctl-defaults.yaml"
)

var (
	ErrSourceDirDoesNotExist = errors.New("source directory does not exist")
	ErrParsingFlag           = errors.New("error while parsing flag")
)

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

			defaultsFilePath := filepath.Join(res.RepoPath, defaultsFileName)

			distributionFile, err := yamlx.FromFileV2[map[any]any](defaultsFilePath)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("%s - %w", defaultsFilePath, err)
			}

			furyctlFile, err := yamlx.FromFileV2[map[any]any](flags.FuryctlPath)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("%s - %w", flags.FuryctlPath, err)
			}

			sourcePath := filepath.Join(res.RepoPath, source)

			if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
				cmdEvent.AddErrorMessage(ErrSourceDirDoesNotExist)
				tracker.Track(cmdEvent)

				return ErrSourceDirDoesNotExist
			}

			merger := merge.NewMerger(
				merge.NewDefaultModel(distributionFile, ".data"),
				merge.NewDefaultModel(furyctlFile, ".spec.distribution"),
			)

			_, err = merger.Merge()
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error merging files: %w", err)
			}

			reverseMerger := merge.NewMerger(
				*merger.GetCustom(),
				*merger.GetBase(),
			)

			_, err = reverseMerger.Merge()
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error merging files: %w", err)
			}

			tmplCfg, err := template.NewConfig(reverseMerger, reverseMerger, []string{})
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error creating template config: %w", err)
			}

			outYaml, err := yamlx.MarshalV2(tmplCfg)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error marshaling template config: %w", err)
			}

			outDirPath, err := os.MkdirTemp("", "furyctl-dist-")
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error creating temporary directory: %w", err)
			}

			confPath := filepath.Join(outDirPath, "config.yaml")

			logrus.Debugf("config path = %s", confPath)

			if err = os.WriteFile(confPath, outYaml, os.ModePerm); err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error writing config file: %w", err)
			}

			if !flags.NoOverwrite {
				if err = os.RemoveAll(flags.OutDir); err != nil {
					cmdEvent.AddErrorMessage(err)
					tracker.Track(cmdEvent)

					return fmt.Errorf("error removing target directory: %w", err)
				}
			}

			templateModel, err := template.NewTemplateModel(
				sourcePath,
				flags.OutDir,
				confPath,
				outDirPath,
				suffix,
				flags.NoOverwrite,
				flags.DryRun,
			)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error creating template model: %w", err)
			}

			err = templateModel.Generate()
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error generating from template: %w", err)
			}

			cmdEvent.AddSuccessMessage("Distribution template generated successfully")
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
