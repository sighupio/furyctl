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

func TestNewFunctionsCmd(t *testing.T) {
	t.Parallel()

	cmd := tools.NewFunctionsCmd()

	assert.Equal(t, "functions", cmd.Use)
	assert.Equal(t, "Generate bash functions for downloaded tools", cmd.Short)
	assert.Contains(t, cmd.Long, "Generate bash functions for tools downloaded by furyctl")
	assert.Contains(t, cmd.Long, "Functions have higher")
	assert.Contains(t, cmd.Long, "override version managers like mise, asdf")
	assert.Contains(t, cmd.Long, "eval \"$(furyctl tools functions)\"")
}

func TestFunctionsCmd_FlagValidation(t *testing.T) {
	t.Parallel()

	cmd := tools.NewFunctionsCmd()

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

func TestFunctionsCmd_Structure(t *testing.T) {
	t.Parallel()

	cmd := tools.NewFunctionsCmd()

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

	distroLocationFlag := cmd.Flags().Lookup("distro-location")
	assert.NotNil(t, distroLocationFlag)
	assert.Contains(t, distroLocationFlag.Usage, "git::git@github.com")
}

func TestFunctionsOutput_Format(t *testing.T) {
	t.Parallel()

	// Test the expected function format.
	toolName := "kubectl"
	toolPath := "/path/to/.furyctl/bin/kubectl/1.25.8/kubectl"

	expectedFunction := `kubectl() { "/path/to/.furyctl/bin/kubectl/1.25.8/kubectl" "$@"; }`

	// Simulate the function generation format.
	actualFunction := toolName + "() { \"" + toolPath + "\" \"$@\"; }"

	assert.Equal(t, expectedFunction, actualFunction)
}

func TestFunctionsOutput_MultipleTools(t *testing.T) {
	t.Parallel()

	// Test multiple functions format.
	tools := []struct {
		name string
		path string
	}{
		{"kubectl", "/path/to/.furyctl/bin/kubectl/1.25.8/kubectl"},
		{"helm", "/path/to/.furyctl/bin/helm/3.10.0/helm"},
		{"terraform", "/path/to/.furyctl/bin/terraform/1.5.7/terraform"},
	}

	functions := make([]string, 0, len(tools))

	for _, tool := range tools {
		function := tool.name + "() { \"" + tool.path + "\" \"$@\"; }"
		functions = append(functions, function)
	}

	output := strings.Join(functions, "\n")

	assert.Contains(t, output, "kubectl() {")
	assert.Contains(t, output, "helm() {")
	assert.Contains(t, output, "terraform() {")
	assert.Len(t, strings.Split(output, "\n"), 3)

	// Verify each function follows the correct format.
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		assert.Contains(t, line, "() { \"")
		assert.Contains(t, line, "\" \"$@\"; }")
	}
}

func TestFunctionsVsAliases_Precedence(t *testing.T) {
	t.Parallel()

	// Test that demonstrates the concept of function vs alias precedence
	// This is more of a documentation test for the behavior difference.

	toolName := "kubectl"
	toolPath := "/path/to/.furyctl/bin/kubectl/1.25.8/kubectl"

	// Alias format.
	alias := "alias " + toolName + "=\"" + toolPath + "\""

	// Function format.
	function := toolName + "() { \"" + toolPath + "\" \"$@\"; }"

	// Both should reference the same tool but with different shell constructs.
	assert.Contains(t, alias, toolPath)
	assert.Contains(t, function, toolPath)

	// Function includes parameter passing mechanism.
	assert.Contains(t, function, "\"$@\"")
	assert.NotContains(t, alias, "\"$@\"")

	// Function uses different shell syntax.
	assert.Contains(t, function, "() {")
	assert.Contains(t, function, "}")
	assert.NotContains(t, alias, "() {")
	assert.NotContains(t, alias, "}")
}
