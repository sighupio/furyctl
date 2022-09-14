package cmd

import (
	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/cmd/validate"
)

var (
	validateCmd = &cobra.Command{
		Use:   "validate",
		Short: "Validate fury config files and dependencies",
	}
)

func init() {
	validateCmd.AddCommand(validate.NewConfigCmd(version))
}
