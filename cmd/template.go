package cmd

import (
	"fmt"
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
			confFile := "conf.yaml"
			suffix := ".tpl"

			templateModel, err := template.NewTemplateModel(source, target, confFile, suffix, false)
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
