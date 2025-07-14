// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tools

import "github.com/spf13/cobra"

// NewFunctionsCmd creates the functions command.
func NewFunctionsCmd() *cobra.Command {
	return CreateShellIntegrationCommand(
		"functions",
		"Generate bash functions for downloaded tools",
		`Generate bash functions for tools downloaded by furyctl.

This command generates bash functions instead of aliases. Functions have higher
precedence than aliases and will override version managers like mise, asdf, or
other tools that create aliases.

Use this when you have version managers installed but want furyctl-downloaded
tools to take precedence.

Examples:
  # View functions
  furyctl tools functions

  # Set functions in current session (overrides mise/asdf)
  eval "$(furyctl tools functions)"

  # Add to your shell profile
  furyctl tools functions >> ~/.bashrc`,
		FunctionFormat,
	)
}
