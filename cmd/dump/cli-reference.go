// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dump

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sighupio/furyctl/internal/analytics"
	"github.com/sighupio/furyctl/internal/app"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
	"github.com/spf13/viper"
)

type CliReferenceCmdFlags struct {
	NoOverwrite bool
	Workdir     string
}

var ErrOutputFolderArgMissing = errors.New("exactly one output folder path argument is allowed")

func NewDumpCLIReferenceCmd() *cobra.Command {
	var cmdEvent analytics.Event

	dumpCLIReferenceCmd := &cobra.Command{
		Use:     "cli-reference <folder>",
		Short:   "Exports the CLI reference in markdown format into a specified folder in the working directory",
		Long:    "Exports the CLI reference in markdown format into a specified folder in the working directory. The folder will be created if it does not exist.",
		Example: `furyctl dump cli-reference ./docs/cli-reference`,
		PreRun: func(cmd *cobra.Command, _ []string) {
			cmdEvent = analytics.NewCommandEvent(cobrax.GetFullname(cmd))

			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				logrus.Fatalf("error while binding flags: %v", err)
			}
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctn := app.GetContainerInstance()

			tracker := ctn.Tracker()
			tracker.Flush()

			flags, err := getDumpCliReferenceCmdFlags()
			if err != nil {
				return err
			}

			const outputFolderPerms = 0o755
			outputFolder := flags.Workdir

			if len(args) > 1 {
				return ErrOutputFolderArgMissing
			} else if len(args) == 1 {
				outputFolder = filepath.Join(outputFolder, args[0])
			}

			if flags.NoOverwrite {
				if _, err := os.Stat(outputFolder); err == nil {
					return fmt.Errorf("output folder %s already exists, use --no-overwrite=false to overwrite it", outputFolder)
				}
			}

			if err := os.MkdirAll(outputFolder, outputFolderPerms); err != nil {
				return fmt.Errorf("failed to create output folder: %w", err)
			}
			outputPath := fmt.Sprintf("%s/index.md", outputFolder)
			mainFile, err := os.Create(outputPath)
			if err != nil {
				return fmt.Errorf("failed to generate CLI reference for main command: %w", err)
			}
			defer mainFile.Close()

			cmd.Root().DisableAutoGenTag = true
			if err := doc.GenMarkdown(cmd.Root(), mainFile); err != nil {
				return fmt.Errorf("failed to generate CLI reference: %w", err)
			}
			for _, command := range cmd.Root().Commands() {
				outputPath := fmt.Sprintf("%s/%s", outputFolder, command.Name())
				if err := os.MkdirAll(outputPath, outputFolderPerms); err != nil {
					return fmt.Errorf("failed to create output folder: %w", err)
				}
				err := doc.GenMarkdownTree(command, outputPath)
				if err != nil {
					return fmt.Errorf("failed to generate CLI reference for command %s: %w", command.Name(), err)
				}
				if err := os.Rename(fmt.Sprintf("%s/furyctl_%s.md", outputPath, command.Name()), fmt.Sprintf("%s/index.md", outputPath)); err != nil {
					return fmt.Errorf("failed to rename CLI reference file for command %s: %w", command.Name(), err)
				}
			}

			logrus.Infof("Markdown CLI reference successfully exported to %s", outputFolder)

			cmdEvent.AddSuccessMessage("CIL Reference generated successfully")
			tracker.Track(cmdEvent)

			return nil
		},
	}

	dumpCLIReferenceCmd.Flags().Bool("no-overwrite", true, "Do not overwrite existing files. Will exit if the output folder already exists")
	dumpCLIReferenceCmd.Flags().StringP("workdir", "w", "", "Working directory to use for the output folder. Default is the current working directory")

	return dumpCLIReferenceCmd
}

func getDumpCliReferenceCmdFlags() (CliReferenceCmdFlags, error) {
	return CliReferenceCmdFlags{
		NoOverwrite: viper.GetBool("no-overwrite"),
		Workdir:     viper.GetString("workdir"),
	}, nil
}
