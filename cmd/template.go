// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"github.com/sighupio/furyctl/internal/merge"
	yaml2 "github.com/sighupio/furyctl/internal/yaml"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/internal/template"
)

var (
	tDryRun bool

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

			distributionFile, err := yaml2.FromFile[map[string]interface{}](distributionFilePath)
			if err != nil {
				return err
			}

			furyctlFile, err := yaml2.FromFile[map[string]interface{}](furyctlFilePath)
			if err != nil {
				return err
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

			confPath := outDirPath + "/config.yaml"

			logrus.Debugf("config path = %s", confPath)

			err = os.WriteFile(confPath, outYaml, os.ModePerm)
			if err != nil {
				return err
			}

			templateModel, err := template.NewTemplateModel(
				source,
				target,
				confPath,
				outDirPath,
				suffix,
				false,
				tDryRun,
			)
			if err != nil {
				return err
			}

			dss, _ := cmd.Flags().GetStringSlice("datasource")

			if len(dss) > 0 {
				if templateModel.Config.Include == nil {
					templateModel.Config.Include = make(map[string]string)
				}
				for _, v := range dss {
					parts := strings.Split(v, "=")
					if len(parts) != 2 {
						return fmt.Errorf("datasource must be given in a form of name=pathToFile")
					}
					templateModel.Config.Include[parts[0]] = parts[1]
				}
			}

			return templateModel.Generate()
		},
	}
)

func init() {
	rootCmd.AddCommand(TemplateCmd)
	TemplateCmd.PersistentFlags().BoolVar(&tDryRun, "dry-run", false, "Dry run execution")
}
