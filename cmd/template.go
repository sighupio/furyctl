// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
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
		Short: "This is a POC for furyctl's Template Engine in go.",
		Long:  `This is a POC for furyctl's Template Engine in go.`,
		RunE: func(cmd *cobra.Command, args []string) error {

			//TODO: Hardcoded for now, we have to think a final strategy for them.
			source := "source"
			target := "target"
			suffix := ".tpl"
			distributionFilePath := "distribution.yaml"
			furyctlFilePath := "furyctl.yaml"

			distributionFile, err := yaml2.FromFile[map[string]any](distributionFilePath)
			if err != nil {
				logrus.Errorf("%s - %+v", distributionFilePath, err)
				return nil
			}

			furyctlFile, err := yaml2.FromFile[map[string]any](furyctlFilePath)
			if err != nil {
				logrus.Errorf("%s - %+v", furyctlFilePath, err)
				return nil
			}

			if _, err := os.Stat(source); os.IsNotExist(err) {
				logrus.Errorf("source directory does not exist")
				return nil
			}

			merger := merge.NewMerger(
				merge.NewDefaultModel(distributionFile, ".data"),
				merge.NewDefaultModel(furyctlFile, ".spec.distribution"),
			)

			mergedDistribution, err := merger.Merge()
			if err != nil {
				logrus.Errorf("%+v", err)
				return nil
			}

			outYaml, err := yaml.Marshal(mergedDistribution)
			if err != nil {
				logrus.Errorf("%+v", err)
				return nil
			}

			outDirPath, err := os.MkdirTemp("", "furyctl-dist-")
			if err != nil {
				logrus.Errorf("%+v", err)
				return nil
			}

			confPath := filepath.Join(outDirPath, "config.yaml")

			logrus.Debugf("config path = %s", confPath)

			if err = os.WriteFile(confPath, outYaml, os.ModePerm); err != nil {
				logrus.Errorf("%+v", err)
				return nil
			}

			if !tNoOverwrite {
				if err = os.RemoveAll(target); err != nil {
					logrus.Errorf("%+v", err)
					return nil
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
				logrus.Errorf("%+v", err)
				return nil
			}

			if err = templateModel.Generate(); err != nil {
				logrus.Errorf("%+v", err)
				return nil
			}

			return nil
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
