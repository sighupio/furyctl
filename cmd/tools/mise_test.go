// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tools_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/cmd/tools"
)

func TestNewMiseCmd(t *testing.T) {
	t.Parallel()

	cmd := tools.NewMiseCmd()

	assert.Equal(t, "mise", cmd.Use)
	assert.Equal(t, "Generate or update mise.toml with downloaded tool paths", cmd.Short)
	assert.Contains(t, cmd.Long, "Generate or update mise.toml configuration file")
	assert.Contains(t, cmd.Long, "furyctl-downloaded tools")
	assert.Contains(t, cmd.Long, "mise install    # No downloads needed")
}

func TestMiseCmd_FlagValidation(t *testing.T) {
	t.Parallel()

	cmd := tools.NewMiseCmd()

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

func TestMiseCmd_Structure(t *testing.T) {
	t.Parallel()

	cmd := tools.NewMiseCmd()

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

func TestMiseConfig_NewFile(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	miseFile := filepath.Join(tempDir, "mise.toml")

	// Simulate creating a new mise.toml file.
	tools := []tools.ToolInfo{
		{
			Name:       "kubectl",
			Version:    "1.25.8",
			BinaryPath: "/path/to/.furyctl/bin/kubectl/1.25.8/kubectl",
			BinaryName: "kubectl",
		},
		{
			Name:       "helm",
			Version:    "3.10.0",
			BinaryPath: "/path/to/.furyctl/bin/helm/3.10.0/helm",
			BinaryName: "helm",
		},
	}

	// Create expected TOML content.
	expected := map[string]interface{}{
		"tools": map[string]interface{}{
			"kubectl": "path:/path/to/.furyctl/bin/kubectl/1.25.8",
			"helm":    "path:/path/to/.furyctl/bin/helm/3.10.0",
		},
	}

	// Simulate what the mise command would generate.
	actualTools := make(map[string]string)

	for _, tool := range tools {
		toolDir := filepath.Dir(tool.BinaryPath)
		actualTools[tool.Name] = "path:" + toolDir
	}

	actualConfig := map[string]interface{}{
		"tools": map[string]interface{}{
			"kubectl": actualTools["kubectl"],
			"helm":    actualTools["helm"],
		},
	}

	// Verify the structure matches expected.
	assert.Equal(t, expected, actualConfig)

	// Write and verify TOML format.
	data, err := toml.Marshal(actualConfig)
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(miseFile, data, 0o644))

	// Read back and verify.
	content, err := os.ReadFile(miseFile)
	require.NoError(t, err)

	var parsed map[string]interface{}

	require.NoError(t, toml.Unmarshal(content, &parsed))

	assert.Equal(t, expected, parsed)
}

func TestMiseConfig_UpdateExisting(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	miseFile := filepath.Join(tempDir, "mise.toml")

	// Create existing mise.toml with other configurations.
	existingConfig := map[string]interface{}{
		"tools": map[string]interface{}{
			"node":   "20.0.0",
			"python": "3.11.0",
		},
		"env": map[string]interface{}{
			"NODE_ENV": "development",
		},
	}

	existingData, err := toml.Marshal(existingConfig)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(miseFile, existingData, 0o644))

	// New tools from furyctl.
	newTools := []tools.ToolInfo{
		{
			Name:       "kubectl",
			Version:    "1.25.8",
			BinaryPath: "/path/to/.furyctl/bin/kubectl/1.25.8/kubectl",
			BinaryName: "kubectl",
		},
		{
			Name:       "terraform",
			Version:    "1.5.7",
			BinaryPath: "/path/to/.furyctl/bin/terraform/1.5.7/terraform",
			BinaryName: "terraform",
		},
	}

	// Simulate loading existing config.
	var loadedConfig map[string]interface{}

	existingContent, err := os.ReadFile(miseFile)
	require.NoError(t, err)
	require.NoError(t, toml.Unmarshal(existingContent, &loadedConfig))

	// Update tools section while preserving other sections.
	toolsSection := make(map[string]interface{})

	if existing, ok := loadedConfig["tools"]; ok {
		if existingTools, ok := existing.(map[string]interface{}); ok {
			for k, v := range existingTools {
				toolsSection[k] = v
			}
		}
	}

	// Add new furyctl tools.
	for _, tool := range newTools {
		toolDir := filepath.Dir(tool.BinaryPath)
		toolsSection[tool.Name] = "path:" + toolDir
	}

	// Update the config.
	loadedConfig["tools"] = toolsSection

	// Expected final structure.
	expected := map[string]interface{}{
		"tools": map[string]interface{}{
			"node":      "20.0.0", // Preserved.
			"python":    "3.11.0", // Preserved.
			"kubectl":   "path:/path/to/.furyctl/bin/kubectl/1.25.8",
			"terraform": "path:/path/to/.furyctl/bin/terraform/1.5.7",
		},
		"env": map[string]interface{}{
			"NODE_ENV": "development", // Preserved.
		},
	}

	assert.Equal(t, expected, loadedConfig)

	// Write updated config.
	updatedData, err := toml.Marshal(loadedConfig)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(miseFile, updatedData, 0o644))

	// Verify final file content.
	finalContent, err := os.ReadFile(miseFile)
	require.NoError(t, err)

	var finalConfig map[string]interface{}

	require.NoError(t, toml.Unmarshal(finalContent, &finalConfig))

	assert.Equal(t, expected, finalConfig)
}

func TestMiseConfig_PathFormat(t *testing.T) {
	t.Parallel()

	// Test the path format used by mise.
	binaryPath := "/home/user/.furyctl/bin/kubectl/1.25.8/kubectl"
	expectedDir := "/home/user/.furyctl/bin/kubectl/1.25.8"
	expectedMisePath := "path:/home/user/.furyctl/bin/kubectl/1.25.8"

	actualDir := filepath.Dir(binaryPath)
	actualMisePath := "path:" + actualDir

	assert.Equal(t, expectedDir, actualDir)
	assert.Equal(t, expectedMisePath, actualMisePath)
}

func TestMiseConfig_TOMLFormat(t *testing.T) {
	t.Parallel()

	// Test TOML marshaling and unmarshaling.
	config := map[string]interface{}{
		"tools": map[string]interface{}{
			"kubectl":   "path:/path/to/kubectl/dir",
			"terraform": "path:/path/to/terraform/dir",
		},
	}

	// Marshal to TOML.
	data, err := toml.Marshal(config)
	require.NoError(t, err)

	tomlStr := string(data)
	assert.Contains(t, tomlStr, "[tools]")
	assert.Contains(t, tomlStr, "kubectl = ")
	assert.Contains(t, tomlStr, "terraform = ")

	// Unmarshal back.
	var parsed map[string]interface{}

	require.NoError(t, toml.Unmarshal(data, &parsed))

	assert.Equal(t, config, parsed)
}

func TestMiseConfig_EmptyFile(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	miseFile := filepath.Join(tempDir, "mise.toml")

	// Create empty file.
	require.NoError(t, os.WriteFile(miseFile, []byte(""), 0o644))

	// Should handle empty file gracefully.
	content, err := os.ReadFile(miseFile)
	require.NoError(t, err)

	var config map[string]interface{}
	err = toml.Unmarshal(content, &config)
	// Empty file should either parse to empty map or return specific error.
	if err != nil { //nolint:wsl // gofumpt and wsl disagree on formatting
		// It's acceptable for empty file to fail parsing.
		assert.Contains(t, strings.ToLower(err.Error()), "empty")
	} else {
		// Or parse to empty config.
		assert.Empty(t, config)
	}
}

func TestMiseCmd_RevertFlagValidation(t *testing.T) {
	t.Parallel()

	cmd := tools.NewMiseCmd()

	// Verify revert flag exists.
	revertFlag := cmd.Flags().Lookup("revert")
	assert.NotNil(t, revertFlag)
	assert.Equal(t, "false", revertFlag.DefValue)
	assert.Equal(t, "Remove furyctl-managed tools from mise.toml instead of adding them", revertFlag.Usage)

	// Verify force flag exists.
	forceFlag := cmd.Flags().Lookup("force")
	assert.NotNil(t, forceFlag)
	assert.Equal(t, "false", forceFlag.DefValue)
	assert.Equal(t, "Skip confirmation prompt when reverting tools", forceFlag.Usage)
}

func TestIdentifyFuryctlTools(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		tools          []tools.ToolInfo
		existingConfig map[string]string
		expected       []string
	}{
		{
			name: "no tools match",
			tools: []tools.ToolInfo{
				{Name: "kubectl", Version: "1.25.8"},
				{Name: "helm", Version: "3.10.0"},
			},
			existingConfig: map[string]string{
				"node":   "20.0.0",
				"python": "3.11.0",
			},
			expected: []string{},
		},
		{
			name: "some tools match",
			tools: []tools.ToolInfo{
				{Name: "kubectl", Version: "1.25.8"},
				{Name: "helm", Version: "3.10.0"},
				{Name: "terraform", Version: "1.5.7"},
			},
			existingConfig: map[string]string{
				"kubectl":   "path:/path/to/kubectl",
				"node":      "20.0.0",
				"terraform": "path:/path/to/terraform",
			},
			expected: []string{"kubectl", "terraform"},
		},
		{
			name: "all tools match",
			tools: []tools.ToolInfo{
				{Name: "kubectl", Version: "1.25.8"},
				{Name: "helm", Version: "3.10.0"},
			},
			existingConfig: map[string]string{
				"kubectl": "path:/path/to/kubectl",
				"helm":    "path:/path/to/helm",
			},
			expected: []string{"kubectl", "helm"},
		},
		{
			name:           "empty tools list",
			tools:          []tools.ToolInfo{},
			existingConfig: map[string]string{"kubectl": "path:/path/to/kubectl"},
			expected:       []string{},
		},
		{
			name: "empty config",
			tools: []tools.ToolInfo{
				{Name: "kubectl", Version: "1.25.8"},
			},
			existingConfig: map[string]string{},
			expected:       []string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			config := &tools.MiseConfig{
				Tools: tc.existingConfig,
				Other: make(map[string]any),
			}

			result := tools.IdentifyFuryctlTools(tc.tools, config)
			assert.ElementsMatch(t, tc.expected, result)
		})
	}
}

func TestRevertMiseConfig_NoFile(t *testing.T) { //nolint:paralleltest // Test changes working directory
	tempDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)

	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tempDir))

	toolsList := []tools.ToolInfo{
		{Name: "kubectl", Version: "1.25.8"},
	}

	// Mock command for output capture.
	cmd := tools.NewMiseCmd()

	err = tools.RevertMiseConfig(toolsList, tools.RevertOptions{SkipConfirmation: true}, cmd)
	require.NoError(t, err)
}

func TestRevertMiseConfig_NoFuryctlTools(t *testing.T) { //nolint:paralleltest // Test changes working directory
	tempDir := t.TempDir()
	miseFile := filepath.Join(tempDir, "mise.toml")
	oldWd, err := os.Getwd()
	require.NoError(t, err)

	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tempDir))

	// Create mise.toml with non-furyctl tools.
	config := map[string]interface{}{
		"tools": map[string]interface{}{
			"node":   "20.0.0",
			"python": "3.11.0",
		},
	}

	data, err := toml.Marshal(config)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(miseFile, data, 0o644))

	toolsList := []tools.ToolInfo{
		{Name: "kubectl", Version: "1.25.8"},
		{Name: "helm", Version: "3.10.0"},
	}

	cmd := tools.NewMiseCmd()

	err = tools.RevertMiseConfig(toolsList, tools.RevertOptions{SkipConfirmation: true}, cmd)
	require.NoError(t, err)

	// Verify original file is unchanged.
	content, err := os.ReadFile(miseFile)
	require.NoError(t, err)

	var finalConfig map[string]interface{}

	require.NoError(t, toml.Unmarshal(content, &finalConfig))

	assert.Equal(t, config, finalConfig)
}

func TestRevertMiseConfig_WithFuryctlTools(t *testing.T) { //nolint:paralleltest // Test changes working directory
	tempDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)

	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tempDir))

	// Create mise.toml with mixed tools.
	config := map[string]interface{}{
		"tools": map[string]interface{}{
			"kubectl":   "path:/path/to/kubectl",
			"helm":      "path:/path/to/helm",
			"node":      "20.0.0",
			"python":    "3.11.0",
			"terraform": "path:/path/to/terraform",
		},
		"env": map[string]interface{}{
			"NODE_ENV": "development",
		},
	}

	data, err := toml.Marshal(config)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile("mise.toml", data, 0o644))

	// Tools that furyctl discovered.
	discoveredTools := []tools.ToolInfo{
		{Name: "kubectl", Version: "1.25.8"},
		{Name: "helm", Version: "3.10.0"},
		{Name: "yq", Version: "4.34.1"}, // Not in mise.toml, should be ignored.
	}

	cmd := tools.NewMiseCmd()

	err = tools.RevertMiseConfig(discoveredTools, tools.RevertOptions{SkipConfirmation: true}, cmd)
	require.NoError(t, err)

	// Verify only furyctl tools were removed.
	content, err := os.ReadFile("mise.toml")
	require.NoError(t, err)

	var finalConfig map[string]interface{}

	require.NoError(t, toml.Unmarshal(content, &finalConfig))

	expected := map[string]interface{}{
		"tools": map[string]interface{}{
			"node":      "20.0.0",
			"python":    "3.11.0",
			"terraform": "path:/path/to/terraform", // Not discovered, so preserved.
		},
		"env": map[string]interface{}{
			"NODE_ENV": "development",
		},
	}

	assert.Equal(t, expected, finalConfig)
}

func TestRevertMiseConfig_PreservesStructure(t *testing.T) { //nolint:paralleltest // Test changes working directory
	tempDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)

	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tempDir))

	// Create complex mise.toml with multiple sections.
	config := map[string]interface{}{
		"tools": map[string]interface{}{
			"kubectl": "path:/path/to/kubectl",
			"node":    "20.0.0",
		},
		"env": map[string]interface{}{
			"NODE_ENV": "development",
			"PATH":     "/custom/path",
		},
		"settings": map[string]interface{}{
			"experimental": true,
			"verbose":      2,
		},
		"alias": map[string]interface{}{
			"k": "kubectl",
		},
	}

	data, err := toml.Marshal(config)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile("mise.toml", data, 0o644))

	toolsList := []tools.ToolInfo{
		{Name: "kubectl", Version: "1.25.8"},
	}

	cmd := tools.NewMiseCmd()

	err = tools.RevertMiseConfig(toolsList, tools.RevertOptions{SkipConfirmation: true}, cmd)
	require.NoError(t, err)

	// Verify structure is preserved, only tools section modified.
	content, err := os.ReadFile("mise.toml")
	require.NoError(t, err)

	var finalConfig map[string]interface{}

	require.NoError(t, toml.Unmarshal(content, &finalConfig))

	expected := map[string]interface{}{
		"tools": map[string]interface{}{
			"node": "20.0.0",
		},
		"env": map[string]interface{}{
			"NODE_ENV": "development",
			"PATH":     "/custom/path",
		},
		"settings": map[string]interface{}{
			"experimental": true,
			"verbose":      int64(2), // TOML unmarshaling converts to int64.
		},
		"alias": map[string]interface{}{
			"k": "kubectl",
		},
	}

	assert.Equal(t, expected, finalConfig)
}

func TestRevertMiseConfig_EmptyToolsSection(t *testing.T) { //nolint:paralleltest // Test changes working directory
	tempDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)

	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tempDir))

	// Create mise.toml with single furyctl tool.
	config := map[string]interface{}{
		"tools": map[string]interface{}{
			"kubectl": "path:/path/to/kubectl",
		},
		"env": map[string]interface{}{
			"NODE_ENV": "development",
		},
	}

	data, err := toml.Marshal(config)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile("mise.toml", data, 0o644))

	toolsList := []tools.ToolInfo{
		{Name: "kubectl", Version: "1.25.8"},
	}

	cmd := tools.NewMiseCmd()

	err = tools.RevertMiseConfig(toolsList, tools.RevertOptions{SkipConfirmation: true}, cmd)
	require.NoError(t, err)

	// Verify tools section is removed when empty, other sections preserved.
	content, err := os.ReadFile("mise.toml")
	require.NoError(t, err)

	var finalConfig map[string]interface{}

	require.NoError(t, toml.Unmarshal(content, &finalConfig))

	expected := map[string]interface{}{
		"env": map[string]interface{}{
			"NODE_ENV": "development",
		},
	}

	assert.Equal(t, expected, finalConfig)
}
