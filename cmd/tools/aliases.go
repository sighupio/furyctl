// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tools

import "github.com/spf13/cobra"

// NewAliasesCmd creates the aliases command.
func NewAliasesCmd() *cobra.Command {
	return CreateShellIntegrationCommand(
		"aliases",
		"Generate bash aliases for downloaded tools",
		`Generate bash aliases for tools downloaded by furyctl.

This command outputs bash aliases that point to the downloaded tool binaries
with versions that are compatible with your SIGHUP Distribution configuration.

Examples:
  # View aliases
  furyctl tools aliases

  # Set aliases in current session
  eval "$(furyctl tools aliases)"

  # Add to your shell profile
  furyctl tools aliases >> ~/.bashrc

  # Use the tools natively
  kubectl --help
  helm list --help`,
		AliasFormat,
	)
}
