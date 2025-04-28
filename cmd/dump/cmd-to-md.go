// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dump

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// Code blow is adapted from:
// https://github.com/spf13/cobra/blob/main/doc/md_docs.go
// https://github.com/spf13/cobra/blob/main/doc/util.go

// Copyright 2013-2023 The Cobra Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// Copyright 2013-2023 The Cobra Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

const markdownExtension = ".md"

func printOptions(buf *bytes.Buffer, cmd *cobra.Command, _ string) error {
	flags := cmd.NonInheritedFlags()
	flags.SetOutput(buf)

	if flags.HasAvailableFlags() {
		if _, err := buf.WriteString("## Options\n\n```bash\n"); err != nil {
			return fmt.Errorf("error while writing to buffer: %w", err)
		}

		flags.PrintDefaults()

		if _, err := buf.WriteString("```\n\n"); err != nil {
			return fmt.Errorf("error while writing to buffer: %w", err)
		}
	}

	parentFlags := cmd.InheritedFlags()
	parentFlags.SetOutput(buf)

	if parentFlags.HasAvailableFlags() {
		if _, err := buf.WriteString("## Options inherited from parent commands\n\n```bash\n"); err != nil {
			return fmt.Errorf("error while writing to buffer: %w", err)
		}

		parentFlags.PrintDefaults()

		if _, err := buf.WriteString("```\n\n"); err != nil {
			return fmt.Errorf("error while writing to buffer: %w", err)
		}
	}

	return nil
}

// GenMarkdown creates markdown output.
func GenMarkdown(cmd *cobra.Command, w io.Writer) error {
	return GenMarkdownCustom(cmd, w, func(s string) string { return s })
}

// GenMarkdownCustom creates custom markdown output.
func GenMarkdownCustom(cmd *cobra.Command, w io.Writer, linkHandler func(string) string) error {
	cmd.InitDefaultHelpCmd()
	cmd.InitDefaultHelpFlag()

	buf := new(bytes.Buffer)
	name := cmd.CommandPath()

	if _, err := buf.WriteString("# " + name + "\n\n"); err != nil {
		return fmt.Errorf("error while writing to buffer: %w", err)
	}

	if _, err := buf.WriteString(cmd.Short + "\n\n"); err != nil {
		return fmt.Errorf("error while writing to buffer: %w", err)
	}

	if len(cmd.Long) > 0 {
		if _, err := buf.WriteString("## Synopsis\n\n%s"); err != nil {
			return fmt.Errorf("error while writing to buffer: %w", err)
		}

		if _, err := buf.WriteString(strings.ReplaceAll(cmd.Long, "\t", "  ") + "\n\n"); err != nil {
			return fmt.Errorf("error while writing to buffer: %w", err)
		}
	}

	if cmd.Runnable() {
		if _, err := buf.WriteString(fmt.Sprintf("## Usage\n\n```bash\n%s\n```\n\n", strings.ReplaceAll(cmd.UseLine(), "\t", "  "))); err != nil {
			return fmt.Errorf("error while writing to buffer: %w", err)
		}
	}

	if len(cmd.Example) > 0 {
		if _, err := buf.WriteString(fmt.Sprintf("## Examples\n\n```bash\n%s\n```\n\n", strings.ReplaceAll(cmd.Example, "\t", "  "))); err != nil {
			return fmt.Errorf("error while writing to buffer: %w", err)
		}
	}

	if err := printOptions(buf, cmd, name); err != nil {
		return err
	}

	if hasSeeAlso(cmd) {
		if _, err := buf.WriteString("## See Also\n\n"); err != nil {
			return fmt.Errorf("error while writing to buffer: %w", err)
		}

		if cmd.HasParent() {
			parent := cmd.Parent()
			pname := parent.CommandPath()
			link := pname + markdownExtension
			link = strings.ReplaceAll(link, " ", "_")

			if _, err := buf.WriteString(fmt.Sprintf("* [%s](%s) - %s\n", pname, linkHandler(link), parent.Short)); err != nil {
				return fmt.Errorf("error while writing to buffer: %w", err)
			}

			cmd.VisitParents(func(c *cobra.Command) {
				if c.DisableAutoGenTag {
					cmd.DisableAutoGenTag = c.DisableAutoGenTag
				}
			})
		}

		children := cmd.Commands()
		sort.Sort(byName(children))

		for _, child := range children {
			if !child.IsAvailableCommand() || child.IsAdditionalHelpTopicCommand() {
				continue
			}

			cname := name + " " + child.Name()
			link := cname + markdownExtension
			link = strings.ReplaceAll(link, " ", "_")

			if _, err := buf.WriteString(fmt.Sprintf("* [%s](%s) - %s\n", cname, linkHandler(link), child.Short)); err != nil {
				return fmt.Errorf("error while writing to buffer: %w", err)
			}
		}
	}

	if _, err := buf.WriteTo(w); err != nil {
		return fmt.Errorf("error while writing contents to file: %w", err)
	}

	return nil
}

// GenMarkdownTree will generate a markdown page for this command and all
// descendants in the directory given. The header may be nil.
// This function may not work correctly if your command names have `-` in them.
// If you have `cmd` with two subcmds, `sub` and `sub-third`,
// and `sub` has a subcommand called `third`, it is undefined which
// help output will be in the file `cmd-sub-third.1`.
func GenMarkdownTree(cmd *cobra.Command, dir string) error {
	identity := func(s string) string { return s }
	emptyStr := func(_ string) string { return "" }

	return GenMarkdownTreeCustom(cmd, dir, emptyStr, identity)
}

// GenMarkdownTreeCustom is the same as GenMarkdownTree, but
// with custom filePrepender and linkHandler.
func GenMarkdownTreeCustom(cmd *cobra.Command, dir string, filePrepender, linkHandler func(string) string) error {
	for _, c := range cmd.Commands() {
		if !c.IsAvailableCommand() || c.IsAdditionalHelpTopicCommand() {
			continue
		}

		if err := GenMarkdownTreeCustom(c, dir, filePrepender, linkHandler); err != nil {
			return err
		}
	}

	basename := strings.ReplaceAll(cmd.CommandPath(), " ", "_") + markdownExtension
	filename := filepath.Join(dir, basename)
	f, err := os.Create(filename)

	if err != nil {
		return fmt.Errorf("error while creating file: %w", err)
	}

	defer f.Close()

	if _, err := io.WriteString(f, filePrepender(filename)); err != nil {
		return fmt.Errorf("error while writing with prepender: %w", err)
	}

	if err := GenMarkdownCustom(cmd, f, linkHandler); err != nil {
		return fmt.Errorf("error while writing with linkHandler: %w", err)
	}

	return nil
}

func hasSeeAlso(cmd *cobra.Command) bool {
	if cmd.HasParent() {
		return true
	}

	for _, c := range cmd.Commands() {
		if !c.IsAvailableCommand() || c.IsAdditionalHelpTopicCommand() {
			continue
		}

		return true
	}

	return false
}

type byName []*cobra.Command

func (s byName) Len() int           { return len(s) }
func (s byName) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s byName) Less(i, j int) bool { return s[i].Name() < s[j].Name() }
