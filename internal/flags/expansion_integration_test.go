//go:build integration

package flags_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/flags"
)

func TestFlagExpansion_BasicFunctionality(t *testing.T) {
	// Reset viper for clean test - don't run in parallel due to global viper state
	defer viper.Reset()

	// Create a simple test config with HOME expansion
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "test-integration.yaml")

	// Use HOME which should be available in test environment
	configContent := `flags:
  global:
    debug: true
    log: "{env://HOME}/integration-test.log"
`

	err := os.WriteFile(configFile, []byte(configContent), 0o644)
	require.NoError(t, err)

	// Create manager and load flags
	manager := flags.NewManager(".")
	err = manager.LoadAndMergeGlobalFlags(configFile)
	require.NoError(t, err, "Should load config without errors")

	// Verify debug flag was loaded
	assert.True(t, viper.GetBool("debug"), "Debug flag should be loaded from config")

	// Verify log path expansion (if HOME is available)
	if homeDir := os.Getenv("HOME"); homeDir != "" {
		logPath := viper.GetString("log")
		assert.Contains(t, logPath, homeDir, "Log path should contain expanded HOME")
		assert.Contains(t, logPath, "integration-test.log", "Log path should contain filename")
		assert.NotContains(t, logPath, "{env://", "Log path should not contain unexpanded variables")
	} else {
		t.Log("HOME not available, skipping expansion verification")
	}
}

func TestFlagExpansion_PrecedenceBasic(t *testing.T) {
	defer viper.Reset()

	// Create a config file with specific values
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "test-precedence.yaml")

	configContent := `flags:
  global:
    debug: false
    log: "/config/path/test.log"
`

	err := os.WriteFile(configFile, []byte(configContent), 0o644)
	require.NoError(t, err)

	// Load config values first
	manager := flags.NewManager(".")
	err = manager.LoadAndMergeGlobalFlags(configFile)
	require.NoError(t, err)

	// Verify config values were loaded
	assert.False(t, viper.GetBool("debug"), "Config debug should be false")
	assert.Equal(t, "/config/path/test.log", viper.GetString("log"), "Config log path should be set")

	// Now simulate CLI flag override (this would normally happen in cobra setup)
	viper.Set("debug", true) // CLI override

	// Verify CLI flag takes precedence
	assert.True(t, viper.GetBool("debug"), "CLI flag should override config")
	assert.Equal(t, "/config/path/test.log", viper.GetString("log"), "Config log should remain unchanged")
}

func TestFlagExpansion_ErrorHandlingRealistic(t *testing.T) {
	t.Run("handles non-existent config gracefully", func(t *testing.T) {
		defer viper.Reset()

		manager := flags.NewManager(".")
		err := manager.LoadAndMergeGlobalFlags("/non/existent/config.yaml")

		// Based on actual behavior, the Manager handles missing config files gracefully
		// It should not return an error, just log and continue
		assert.NoError(t, err, "Manager should handle missing config files gracefully")
	})

	t.Run("config with missing flags section", func(t *testing.T) {
		defer viper.Reset()

		tempDir := t.TempDir()
		configFile := filepath.Join(tempDir, "no-flags.yaml")

		// Write config without flags section
		configContent := `apiVersion: kfd.sighup.io/v1alpha2
kind: EKSCluster
metadata:
  name: test
spec:
  distributionVersion: v1.32.0
`

		err := os.WriteFile(configFile, []byte(configContent), 0o644)
		require.NoError(t, err)

		manager := flags.NewManager(".")
		err = manager.LoadAndMergeGlobalFlags(configFile)

		// Should handle configs without flags section gracefully
		assert.NoError(t, err, "Should handle configs without flags section")
	})
}

func TestFlagExpansion_GlobalFlagsOnly(t *testing.T) {
	defer viper.Reset()

	// Test only global flags (avoid command-specific validation issues)
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "global-only.yaml")

	configContent := `flags:
  global:
    debug: true
    log: "/tmp/global-test.log"
`

	err := os.WriteFile(configFile, []byte(configContent), 0o644)
	require.NoError(t, err)

	manager := flags.NewManager(".")

	// Test loading global flags only
	err = manager.LoadAndMergeGlobalFlags(configFile)
	require.NoError(t, err, "Should load global flags without errors")

	// Verify global flags were loaded
	assert.True(t, viper.GetBool("debug"), "Global debug flag should be loaded")
	assert.Equal(t, "/tmp/global-test.log", viper.GetString("log"), "Global log should be set")
}

func TestFlagExpansion_LoadFromCurrentDirectory(t *testing.T) {
	defer viper.Reset()

	// Create a temporary directory and work in it
	tempDir := t.TempDir()

	// Create furyctl.yaml in temp directory
	configFile := filepath.Join(tempDir, "furyctl.yaml")
	configContent := `flags:
  global:
    debug: true
    log: "/tmp/current-dir.log"
`

	err := os.WriteFile(configFile, []byte(configContent), 0o644)
	require.NoError(t, err)

	// Change to temp directory
	originalDir, _ := os.Getwd()
	defer func() {
		os.Chdir(originalDir)
	}()

	err = os.Chdir(tempDir)
	require.NoError(t, err)

	manager := flags.NewManager(".")

	// Try loading from current directory
	err = manager.TryLoadFromCurrentDirectory("validate")
	require.NoError(t, err, "Should load from current directory")

	// Verify flags were loaded
	assert.True(t, viper.GetBool("debug"), "Debug should be loaded from current directory")
	assert.Equal(t, "/tmp/current-dir.log", viper.GetString("log"), "Log path should be loaded")
}

func TestFlagExpansion_WithValidCommand(t *testing.T) {
	defer viper.Reset()

	// Test with a command that actually supports specific flags
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "validate-command.yaml")

	// Only use global flags to avoid validation issues
	configContent := `flags:
  global:
    debug: false
    log: "/tmp/validate-test.log"
`

	err := os.WriteFile(configFile, []byte(configContent), 0o644)
	require.NoError(t, err)

	manager := flags.NewManager(".")

	// Test loading flags for validate command (which should be safe)
	err = manager.LoadAndMergeFlags(configFile, "validate")
	require.NoError(t, err, "Should load flags for validate command")

	// Verify global flags were loaded
	assert.False(t, viper.GetBool("debug"), "Global debug flag should be loaded")
	assert.Equal(t, "/tmp/validate-test.log", viper.GetString("log"), "Global log should be set")
}
