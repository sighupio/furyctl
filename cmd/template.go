package cmd

import (
	"fmt"
	"github.com/sighupio/furyctl/internal/merge"
	"gopkg.in/yaml.v3"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/internal/template"
)

var (
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

			distributionFile, err := merge.ReadYAMLfromFile(distributionFilePath)
			if err != nil {
				return err
			}

			furyctlFile, err := merge.ReadYAMLfromFile(furyctlFilePath)
			if err != nil {
				return err
			}

			mergedDistribution, err := merge.Merge(distributionFile, furyctlFile)
			if err != nil {
				return err
			}

			outYaml, err := yaml.Marshal(mergedDistribution)
			if err != nil {
				panic(err)
			}

			outDirPath, err := os.MkdirTemp("", "furyctl-dist-")
			if err != nil {
				return err
			}

			confPath := outDirPath + "/config.yaml"

			err = os.WriteFile(confPath, outYaml, os.ModePerm)
			if err != nil {
				return err
			}

			templateModel, err := template.NewTemplateModel(source, target, confPath, suffix, false)
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
}
