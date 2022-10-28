// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dump

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/internal/merge"
	"github.com/sighupio/furyctl/internal/template"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

var (
	ErrSourceDirDoesNotExist = fmt.Errorf("source directory does not exist")

	ErrDebugFlagNotSet          = fmt.Errorf("debug flag not set")
	ErrConfigFlagNotSet         = fmt.Errorf("config flag not set")
	ErrDistroLocationFlagNotSet = fmt.Errorf("distro-location flag not set")
	ErrBinPathFlagNotSet        = fmt.Errorf("bin-path flag not set")
)

type templateConfig struct {
	DryRun      bool
	NoOverwrite bool
}

func NewTemplateCmd() *cobra.Command {
	cfg := templateConfig{}
	templateCmd := &cobra.Command{
		Use:   "template",
		Short: "Renders the distribution's manifests from a template and a configuration file",
		Long: `Generates a folder with the Kustomization project for deploying Kubernetes Fury Distribution into a cluster.
The generated folder will be created starting from a provided template and the parameters set in a configuration file that is merged with default values.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			source := "source"
			target := "target"
			suffix := ".tpl"
			distributionFilePath := "distribution.yaml"
			furyctlFilePath := "furyctl.yaml"

			distributionFile, err := yamlx.FromFileV2[map[any]any](distributionFilePath)
			if err != nil {
				return fmt.Errorf("%s - %w", distributionFilePath, err)
			}

			furyctlFile, err := yamlx.FromFileV2[map[any]any](furyctlFilePath)
			if err != nil {
				return fmt.Errorf("%s - %w", furyctlFilePath, err)
			}

			if _, err := os.Stat(source); os.IsNotExist(err) {
				return ErrSourceDirDoesNotExist
			}

			merger := merge.NewMerger(
				merge.NewDefaultModel(distributionFile, ".data"),
				merge.NewDefaultModel(furyctlFile, ".spec.distribution"),
			)

			_, err = merger.Merge()
			if err != nil {
				return fmt.Errorf("error merging files: %w", err)
			}

			reverseMerger := merge.NewMerger(
				*merger.GetCustom(),
				*merger.GetBase(),
			)

			_, err = reverseMerger.Merge()
			if err != nil {
				return fmt.Errorf("error merging files: %w", err)
			}

			tmplCfg, err := template.NewConfig(reverseMerger, reverseMerger, []string{})
			if err != nil {
				return fmt.Errorf("error creating template config: %w", err)
			}

			outYaml, err := yamlx.MarshalV2(tmplCfg)
			if err != nil {
				return fmt.Errorf("error marshaling template config: %w", err)
			}

			outDirPath, err := os.MkdirTemp("", "furyctl-dist-")
			if err != nil {
				return fmt.Errorf("error creating temporary directory: %w", err)
			}

			confPath := filepath.Join(outDirPath, "config.yaml")

			logrus.Debugf("config path = %s", confPath)

			if err = os.WriteFile(confPath, outYaml, os.ModePerm); err != nil {
				return fmt.Errorf("error writing config file: %w", err)
			}

			if !cfg.NoOverwrite {
				if err = os.RemoveAll(target); err != nil {
					return fmt.Errorf("error removing target directory: %w", err)
				}
			}

			templateModel, err := template.NewTemplateModel(
				source,
				target,
				confPath,
				outDirPath,
				suffix,
				cfg.NoOverwrite,
				cfg.DryRun,
			)
			if err != nil {
				return fmt.Errorf("error creating template model: %w", err)
			}

			err = templateModel.Generate()
			if err != nil {
				return fmt.Errorf("error generating from template: %w", err)
			}

			return nil
		},
	}

	templateCmd.Flags().BoolVar(
		&cfg.DryRun,
		"dry-run",
		false,
		"Furyctl will try its best to generate the manifests despite the errors",
	)
	templateCmd.Flags().BoolVar(
		&cfg.NoOverwrite,
		"no-overwrite",
		false,
		"Stop if target directory is not empty",
	)

	return templateCmd
}
