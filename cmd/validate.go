package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/cmd/cmdutil"
	"github.com/sighupio/furyctl/cmd/validate"
	"github.com/sighupio/furyctl/internal/schema/santhosh"
)

var (
	errDefaultsDownload = fmt.Errorf("error downloading json schema for furyctl.yaml")

	validateCmd = &cobra.Command{
		Use:   "validate",
		Short: "Validate Furyfile",
		RunE: func(cmd *cobra.Command, _ []string) error {
			debug := cmd.Flag("debug").Value.String() == "true"
			furyctlFilePath := cmd.Flag("config").Value.String()
			schemasLocation := cmd.Flag("schemas-location").Value.String()
			defaultsLocation := cmd.Flag("defaults-location").Value.String()

			schemasPath, err := validate.DownloadFolder(schemasLocation, "schemas")
			if err != nil {
				return err
			}
			if !debug {
				defer validate.CleanupTempDir(filepath.Base(schemasPath))
			}

			defaultsPath, err := validate.DownloadFolder(defaultsLocation, "defaults")
			if err != nil {
				return err
			}
			if !debug {
				defer validate.CleanupTempDir(filepath.Base(defaultsPath))
			}

			hasErrors := error(nil)

			minimalConf := cmdutil.LoadConfig[validate.FuryctlConfig](furyctlFilePath)

			schemaPath := validate.GetSchemaPath(schemasPath, minimalConf)
			defaultPath := validate.GetDefaultPath(defaultsPath, minimalConf)

			defaultedFuryctlFilePath, err := validate.MergeConfigAndDefaults(furyctlFilePath, defaultPath)
			if err != nil {
				return err
			}
			if !debug {
				defer validate.CleanupTempDir(filepath.Base(defaultedFuryctlFilePath))
			}

			schema, err := santhosh.LoadSchema(schemaPath)
			if err != nil {
				return err
			}

			conf := cmdutil.LoadConfig[any](defaultedFuryctlFilePath)

			if err := schema.ValidateInterface(conf); err != nil {
				validate.PrintResults(err, defaultedFuryctlFilePath)

				hasErrors = validate.ErrHasValidationErrors
			}

			validate.PrintSummary(hasErrors != nil)

			return hasErrors
		},
	}
)

func init() {
	validateCmd.Flags().StringP(
		"config",
		"c",
		"furyctl.yaml",
		"Path to the furyctl.yaml file",
	)

	validateCmd.Flags().StringP(
		"schemas-location",
		"",
		"",
		"Base URL used to download schemas. "+
			"It can either be a local path(eg: /path/to/fury/distribution//schemas) or "+
			"a remote URL(eg: https://git@github.com/sighupio/fury-distribution//schemas?ref=BRANCH_NAME)."+
			"Any format supported by hashicorp/go-getter can be used.",
	)

	validateCmd.Flags().StringP(
		"defaults-location",
		"",
		"",
		"Base URL used to download defaults. "+
			"It can either be a local path(eg: /path/to/fury/distribution//defaults) or "+
			"a remote URL(eg: https://git@github.com/sighupio/fury-distribution//defaults?ref=BRANCH_NAME)."+
			"Any format supported by hashicorp/go-getter can be used.",
	)
}
