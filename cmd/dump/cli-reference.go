package dump

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

func NewDumpCLIReferenceCmd() *cobra.Command {
	dumpCLIReferenceCmd := &cobra.Command{
		Use:   "cli-reference <folder>",
		Short: "Exports the CLI reference in markdown format into a specified folder",
		Long:  "Exports the CLI reference in markdown format into a specified folder",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("output folder path is required")
			}

			outputFolder := args[0]
			if err := os.MkdirAll(outputFolder, 0o755); err != nil {
				return fmt.Errorf("failed to create output folder: %w", err)
			}

			err := doc.GenMarkdownTree(cmd.Root(), outputFolder)
			if err != nil {
				return fmt.Errorf("failed to generate CLI reference: %w", err)
			}

			logrus.Infof("Markdown CLI reference successfuly exported to %s", outputFolder)
			return nil
		},
	}

	return dumpCLIReferenceCmd
}
