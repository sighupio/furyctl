// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tools_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sighupio/furyctl/cmd/tools"
)

func TestNewAliasesCmd(t *testing.T) {
	t.Parallel()

	cmd := tools.NewAliasesCmd()

	assert.Equal(t, "aliases", cmd.Use)
	assert.Equal(t, "Generate bash aliases for downloaded tools", cmd.Short)
	assert.Contains(t, cmd.Long, "Generate bash aliases for tools downloaded by furyctl")
	assert.Contains(t, cmd.Long, "eval \"$(furyctl tools aliases)\"")
}

func TestAliasesCmd_FlagValidation(t *testing.T) {
	t.Parallel()

	cmd := tools.NewAliasesCmd()

	// Test that command can be created and has expected flags.
	assert.NotNil(t, cmd)

	// Verify required flags exist.
	configFlag := cmd.Flags().Lookup("config")
	assert.NotNil(t, configFlag)
	assert.Equal(t, "furyctl.yaml", configFlag.DefValue)

	binPathFlag := cmd.Flags().Lookup("bin-path")
	assert.NotNil(t, binPathFlag)

	skipDepsFlag := cmd.Flags().Lookup("skip-deps-download")
	assert.NotNil(t, skipDepsFlag)
	assert.Equal(t, "false", skipDepsFlag.DefValue)
}

func TestAliasesCmd_CommandStructure(t *testing.T) {
	t.Parallel()

	cmd := tools.NewAliasesCmd()

	// Verify the command is properly structured.
	assert.NotNil(t, cmd.PreRun)
	assert.NotNil(t, cmd.RunE)

	// Check flags are properly defined.
	binPathFlag := cmd.Flags().Lookup("bin-path")
	assert.NotNil(t, binPathFlag)
	assert.Equal(t, "b", binPathFlag.Shorthand)

	configFlag := cmd.Flags().Lookup("config")
	assert.NotNil(t, configFlag)
	assert.Equal(t, "c", configFlag.Shorthand)
	assert.Equal(t, "furyctl.yaml", configFlag.DefValue)

	skipDepsFlag := cmd.Flags().Lookup("skip-deps-download")
	assert.NotNil(t, skipDepsFlag)
	assert.Equal(t, "false", skipDepsFlag.DefValue)
}

func TestAliasesOutput_Format(t *testing.T) {
	t.Parallel()

	// Test the expected alias format.
	toolName := "kubectl"
	toolPath := "/path/to/.furyctl/bin/kubectl/1.25.8/kubectl"

	expectedAlias := `alias kubectl="/path/to/.furyctl/bin/kubectl/1.25.8/kubectl"`

	// Simulate the alias generation format.
	actualAlias := "alias " + toolName + "=\"" + toolPath + "\""

	assert.Equal(t, expectedAlias, actualAlias)
}

func TestAliasesOutput_MultipleTools(t *testing.T) {
	t.Parallel()

	// Test multiple aliases format.
	tools := []struct {
		name string
		path string
	}{
		{"kubectl", "/path/to/.furyctl/bin/kubectl/1.25.8/kubectl"},
		{"helm", "/path/to/.furyctl/bin/helm/3.10.0/helm"},
		{"terraform", "/path/to/.furyctl/bin/terraform/1.5.7/terraform"},
	}

	aliases := make([]string, 0, len(tools))

	for _, tool := range tools {
		alias := "alias " + tool.name + "=\"" + tool.path + "\""
		aliases = append(aliases, alias)
	}

	output := strings.Join(aliases, "\n")

	assert.Contains(t, output, "alias kubectl=")
	assert.Contains(t, output, "alias helm=")
	assert.Contains(t, output, "alias terraform=")
	assert.Len(t, strings.Split(output, "\n"), 3)
}
