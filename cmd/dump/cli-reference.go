// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dump

import (
	"errors"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

var ErrOutputFolderArgMissing = errors.New("output folder path argument is required")

func NewDumpCLIReferenceCmd() *cobra.Command {
	dumpCLIReferenceCmd := &cobra.Command{
		Use:   "cli-reference <folder>",
		Short: "Exports the CLI reference in markdown format into a specified folder",
		Long:  "Exports the CLI reference in markdown format into a specified folder",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return ErrOutputFolderArgMissing
			}

			outputFolder := args[0]
			const perms = 0o755
			if err := os.MkdirAll(outputFolder, perms); err != nil {
				return fmt.Errorf("failed to create output folder: %w", err)
			}

			err := doc.GenMarkdownTree(cmd.Root(), outputFolder)
			if err != nil {
				return fmt.Errorf("failed to generate CLI reference: %w", err)
			}

			logrus.Infof("Markdown CLI reference successfully exported to %s", outputFolder)

			return nil
		},
	}

	return dumpCLIReferenceCmd
}
