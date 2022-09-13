package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/cmd/cmdutil"
	"github.com/sighupio/furyctl/cmd/validate"
	"github.com/sighupio/furyctl/internal/schema/santhosh"
)

var ErrSchemaDownload = fmt.Errorf("error downloading json schema for furyctl.yaml")

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate Furyfile",
	RunE: func(cmd *cobra.Command, args []string) error {
		distroLocation := cmd.Flag("distro-location").Value.String()

		schemasPath, err := validate.DownloadSchemas(distroLocation)
		if err != nil {
			return fmt.Errorf("%s: %w", ErrSchemaDownload, err)
		}

		hasErrors := error(nil)
		furyctlFile, err := validate.ParseArgs(args)
		if err != nil {
			return err
		}

		minimalConf := cmdutil.LoadConfig[validate.FuryctlConfig](furyctlFile)

		schemaPath := validate.GetSchemaPath(schemasPath, minimalConf)

		schema, err := santhosh.LoadSchema(schemaPath)
		if err != nil {
			return fmt.Errorf("failed to load schema: %w", err)
		}

		conf := cmdutil.LoadConfig[any](furyctlFile)

		if err := schema.ValidateInterface(conf); err != nil {
			validate.PrintResults(err, conf, furyctlFile)

			hasErrors = validate.ErrHasValidationErrors
		}

		validate.PrintSummary(hasErrors != nil)

		return hasErrors
	},
}

func init() {
	validateCmd.Flags().StringP("distro-location", "l", "", "Base URL used to download schemas.")
	rootCmd.AddCommand(validateCmd)
}
