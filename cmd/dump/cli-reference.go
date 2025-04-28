// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dump

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sighupio/furyctl/internal/analytics"
	"github.com/sighupio/furyctl/internal/app"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
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
		Use:     "cli-reference [folder]",
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

			// nullFilePrepender is a function that generates an empty front matter for the markdown files
			nullFilePrepender := func(filename string) string {
				return ""
			}

			linkHandlerRoot := func(name string) string {
				if name == "furyctl.md" {
					return "index.md"
				}

				for _, command := range cmd.Root().Commands() {
					basename := strings.Replace(command.CommandPath(), " ", "_", -1) + ".md"
					// logrus.Debugf("checking command %s with basename %s", command.Name(), basename)
					if basename == name {
						// logrus.Debugf("found command %s with basename %s", command.Name(), basename)
						return fmt.Sprintf("%s/index.md", command.Name())
					}
				}

				return name
			}

			linkHandler := func(name string) string {
				if name == "furyctl.md" {
					return "../index.md"
				}

				for _, command := range cmd.Root().Commands() {
					basename := strings.Replace(command.CommandPath(), " ", "_", -1) + ".md"
					// logrus.Debugf("checking command %s with basename %s", command.Name(), basename)
					if basename == name {
						// logrus.Debugf("found command %s with basename %s", command.Name(), basename)
						return fmt.Sprintf("../%s/index.md", command.Name())
					}
				}

				return name
			}

			cmd.Root().DisableAutoGenTag = true

			if err := GenMarkdownCustom(cmd.Root(), mainFile, linkHandlerRoot); err != nil {
				return fmt.Errorf("failed to generate CLI reference: %w", err)
			}
			for _, command := range cmd.Root().Commands() {
				outputPath := fmt.Sprintf("%s/%s", outputFolder, command.Name())
				if err := os.MkdirAll(outputPath, outputFolderPerms); err != nil {
					return fmt.Errorf("failed to create output folder: %w", err)
				}
				// err := GenMarkdownTree(command, outputPath)
				err := GenMarkdownTreeCustom(command, outputPath, nullFilePrepender, linkHandler)
				if err != nil {
					return fmt.Errorf("failed to generate CLI reference for command %s: %w", command.Name(), err)
				}
				if err := os.Rename(fmt.Sprintf("%s/furyctl_%s.md", outputPath, command.Name()), fmt.Sprintf("%s/index.md", outputPath)); err != nil {
					return fmt.Errorf("failed to rename CLI reference file for command %s: %w", command.Name(), err)
				}
				// replace lines in the generated markdown that break docusaurus because it thinks are MDX
				if command.Name() == "completion" {
					escapeCodeBlock(fmt.Sprintf("%s/index.md", outputPath))
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

func escapeCodeBlock(path string) error {
	// Escape code blocks in the file sorrounding them with triple backticks
	if content, err := os.ReadFile(path); err != nil {
		return fmt.Errorf("failed to read file %s: %w", path, err)
	} else {
		reUnix := regexp.MustCompile("(.+)\\$ (.+)")
		rePS := regexp.MustCompile("(.+)PS> (.+)")
		escapedContent := string(content)
		escapedContent = reUnix.ReplaceAllString(escapedContent, "${1}```shell\n${1}$$ ${2}\n${1}```")
		escapedContent = rePS.ReplaceAllString(escapedContent, "${1}```powershell\n${1}PS> ${2}\n${1}```")
		if err := os.WriteFile(path, []byte(escapedContent), 0o644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", path, err)
		}
	}

	return nil
}
