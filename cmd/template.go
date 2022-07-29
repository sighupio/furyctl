// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"github.com/sighupio/furyctl/internal/merge"
	yaml2 "github.com/sighupio/furyctl/internal/yaml"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
	"os"
	"path/filepath"

	"github.com/sighupio/furyctl/internal/template"
)

var (
	tDryRun      bool
	tNoOverwrite bool

	TemplateCmd = &cobra.Command{
		Use:   "template",
		Short: "Renders the distribution's manifests from a template and a configuration file",
		Long: `Generates a folder with the Kustomization project for deploying Kubernetes Fury Distribution into a cluster.
The generated folder will be created starting from a provided template and the parameters set in a configuration file that is merged with default values.`,
		RunE: func(cmd *cobra.Command, args []string) error {

			//TODO(rm-2470): To be reworked in redmine task - Define template command flags.
			source := "source"
			target := "target"
			suffix := ".tpl"
			distributionFilePath := "distribution.yaml"
			furyctlFilePath := "furyctl.yaml"

			distributionFile, err := yaml2.FromFile[map[any]any](distributionFilePath)
			if err != nil {
				return fmt.Errorf("%s - %w", distributionFilePath, err)
			}

			furyctlFile, err := yaml2.FromFile[map[any]any](furyctlFilePath)
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

			outYaml, err := yaml.Marshal(mergedDistribution)
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

			if !tNoOverwrite {
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
				tNoOverwrite,
				tDryRun,
			)
			if err != nil {
				return err
			}

			return templateModel.Generate()
		},
	}
)

func init() {
	rootCmd.AddCommand(TemplateCmd)
	TemplateCmd.Flags().BoolVar(&tDryRun, "dry-run", false, "Dry run execution")
	TemplateCmd.Flags().BoolVar(
		&tNoOverwrite,
		"no-overwrite",
		false,
		"Stop if target directory is not empty",
	)
}
