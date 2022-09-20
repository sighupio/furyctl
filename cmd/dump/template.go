// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dump

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sighupio/furyctl/internal/merge"
	"github.com/sighupio/furyctl/internal/yaml"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/internal/template"
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
			// TODO(rm-2470): To be reworked in redmine task - Define template command flags.
			source := "source"
			target := "target"
			suffix := ".tpl"
			distributionFilePath := "distribution.yaml"
			furyctlFilePath := "furyctl.yaml"

			distributionFile, err := yaml.FromFileV2[map[any]any](distributionFilePath)
			if err != nil {
				return fmt.Errorf("%s - %w", distributionFilePath, err)
			}

			furyctlFile, err := yaml.FromFileV2[map[any]any](furyctlFilePath)
			if err != nil {
				return fmt.Errorf("%s - %w", furyctlFilePath, err)
			}

			if _, err := os.Stat(source); os.IsNotExist(err) {
				return fmt.Errorf("source directory does not exist")
			}

			merger := merge.NewMerger(
				merge.NewDefaultModel(distributionFile, ".data"),
				merge.NewDefaultModel(furyctlFile, ".spec.distribution"),
			)

			mergedDistribution, err := merger.Merge()
			if err != nil {
				return err
			}

			outYaml, err := yaml.MarshalV2(mergedDistribution)
			if err != nil {
				return err
			}

			outDirPath, err := os.MkdirTemp("", "furyctl-dist-")
			if err != nil {
				return err
			}

			confPath := filepath.Join(outDirPath, "config.yaml")

			logrus.Debugf("config path = %s", confPath)

			if err = os.WriteFile(confPath, outYaml, os.ModePerm); err != nil {
				return err
			}

			if !cfg.NoOverwrite {
				if err = os.RemoveAll(target); err != nil {
					return err
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
				return err
			}

			return templateModel.Generate()
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
