// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sighupio/furyctl/cmd"
)

func TestNewToolsCmd(t *testing.T) {
	t.Parallel()

	toolsCmd := cmd.NewToolsCmd()

	assert.Equal(t, "tools", toolsCmd.Use)
	assert.Equal(t, "Manage tool integrations for downloaded binaries", toolsCmd.Short)
	assert.Contains(t, toolsCmd.Long, "Manage tool integrations for downloaded binaries")
	assert.Contains(t, toolsCmd.Long, "aliases: Traditional bash aliases")

	// Check that subcommands are added.
	subcommands := toolsCmd.Commands()
	assert.Len(t, subcommands, 3)

	commandNames := make([]string, 0, len(subcommands))
	for _, subcmd := range subcommands {
		commandNames = append(commandNames, subcmd.Use)
	}

	assert.Contains(t, commandNames, "aliases")
	assert.Contains(t, commandNames, "functions")
	assert.Contains(t, commandNames, "mise")
}

func TestToolsCmd_SubcommandStructure(t *testing.T) {
	t.Parallel()

	toolsCmd := cmd.NewToolsCmd()
	subcommands := toolsCmd.Commands()

	// Find each subcommand and verify basic properties.
	for _, subcmd := range subcommands {
		switch subcmd.Use {
		case "aliases":
			assert.Equal(t, "Generate bash aliases for downloaded tools", subcmd.Short)
			assert.Contains(t, subcmd.Long, "bash aliases")

		case "functions":
			assert.Equal(t, "Generate bash functions for downloaded tools", subcmd.Short)
			assert.Contains(t, subcmd.Long, "bash functions")
			assert.Contains(t, subcmd.Long, "Functions have higher")

		case "mise":
			assert.Equal(t, "Generate or update mise.toml with downloaded tool paths", subcmd.Short)
			assert.Contains(t, subcmd.Long, "mise.toml")
		}

		// All subcommands should have the same flags.
		assert.NotNil(t, subcmd.Flags().Lookup("bin-path"))
		assert.NotNil(t, subcmd.Flags().Lookup("config"))
		assert.NotNil(t, subcmd.Flags().Lookup("distro-location"))
		assert.NotNil(t, subcmd.Flags().Lookup("skip-deps-download"))
	}
}

func TestToolsCmd_HelpContent(t *testing.T) {
	t.Parallel()

	toolsCmd := cmd.NewToolsCmd()

	// The help should mention the different output formats.
	assert.Contains(t, toolsCmd.Long, "aliases:")
	assert.Contains(t, toolsCmd.Long, "functions:")
	assert.Contains(t, toolsCmd.Long, "mise:")

	// Should explain the use cases.
	assert.Contains(t, toolsCmd.Long, "mise/asdf")
}
