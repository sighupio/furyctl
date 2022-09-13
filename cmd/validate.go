package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/cmd/validate"
	"github.com/sighupio/furyctl/internal/schema/santhosh"
)

var configPath string
var output string

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate Furyfile",
	RunE: func(cmd *cobra.Command, args []string) error {
		schemaLocation := "https://raw.githubusercontent.com/sighupio/fury-distribution/feature/create-draft-of-the-furyctl-yaml-json-schema"

		schema, err := santhosh.LoadSchema(schemaLocation)
		if err != nil {
			return fmt.Errorf("failed to load schema: %w", err)
		}

		hasErrors := error(nil)

		conf := validate.LoadConfig[validate.FuryDistributionSpecs](configPath)
		if err := schema.ValidateInterface(conf); err != nil {
			validate.PrintResults(output, err, conf, configPath)

			hasErrors = validate.ErrHasValidationErrors
		}

		validate.PrintSummary(output, hasErrors != nil)

		return hasErrors
	},
}

func init() {
	validateCmd.Flags().StringP("config", "c", configPath, "Furyctl config file path")
	validateCmd.Flags().StringP("output", "o", "text", "Output format (text|json)")
	rootCmd.AddCommand(validateCmd)
}
