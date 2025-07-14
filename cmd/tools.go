// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/cmd/tools"
)

func NewToolsCmd() *cobra.Command {
	toolsCmd := &cobra.Command{
		Use:   "tools",
		Short: "Manage tool integrations for downloaded binaries",
		Long: `Manage tool integrations for downloaded binaries.

Generate various output formats for integrating furyctl-downloaded tools 
with your shell environment. Choose the format that works best with your 
development setup:

- aliases: Traditional bash aliases (default)
- functions: Bash functions that override version managers like mise/asdf  
- mise: Native mise integration via mise.toml file`,
	}

	toolsCmd.AddCommand(tools.NewAliasesCmd())
	toolsCmd.AddCommand(tools.NewFunctionsCmd())
	toolsCmd.AddCommand(tools.NewMiseCmd())

	return toolsCmd
}
