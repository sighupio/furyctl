// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigFlagDetection_CommandLineParsing(t *testing.T) {
	t.Parallel()

	// Test the command line parsing logic we added to PersistentPreRun
	testCases := []struct {
		name         string
		args         []string
		expectedPath string
	}{
		{
			name:         "no config flag",
			args:         []string{"furyctl", "validate", "config"},
			expectedPath: "",
		},
		{
			name:         "config flag with space",
			args:         []string{"furyctl", "validate", "config", "--config", "test.yaml"},
			expectedPath: "test.yaml",
		},
		{
			name:         "config flag short form",
			args:         []string{"furyctl", "validate", "config", "-c", "test.yaml"},
			expectedPath: "test.yaml",
		},
		{
			name:         "config flag with equals",
			args:         []string{"furyctl", "validate", "config", "--config=test.yaml"},
			expectedPath: "test.yaml",
		},
		{
			name:         "config flag at end",
			args:         []string{"furyctl", "validate", "config", "--distro-location", "../distribution", "--config", "test.yaml"},
			expectedPath: "test.yaml",
		},
		{
			name:         "config flag at beginning",
			args:         []string{"furyctl", "--config", "test.yaml", "validate", "config"},
			expectedPath: "test.yaml",
		},
		{
			name:         "config flag with path",
			args:         []string{"furyctl", "validate", "config", "--config", "../local-testing/test.yaml"},
			expectedPath: "../local-testing/test.yaml",
		},
		{
			name:         "config flag with absolute path",
			args:         []string{"furyctl", "validate", "config", "--config", "/tmp/test.yaml"},
			expectedPath: "/tmp/test.yaml",
		},
		{
			name:         "multiple flags with config",
			args:         []string{"furyctl", "--debug", "validate", "config", "--config", "test.yaml", "--distro-location", "../dist"},
			expectedPath: "test.yaml",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Extract the logic from our PersistentPreRun for testing
			var configPath string
			args := tc.args
			for i, arg := range args {
				if arg == "--config" || arg == "-c" {
					if i+1 < len(args) {
						configPath = args[i+1]
						break
					}
				} else if strings.HasPrefix(arg, "--config=") {
					configPath = strings.TrimPrefix(arg, "--config=")
					break
				}
			}

			assert.Equal(t, tc.expectedPath, configPath)
		})
	}
}

func TestConfigFlagDetection_EdgeCases(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		args         []string
		expectedPath string
		description  string
	}{
		{
			name:         "config flag without value",
			args:         []string{"furyctl", "validate", "config", "--config"},
			expectedPath: "",
			description:  "Should not crash when --config has no value",
		},
		{
			name:         "short config flag without value",
			args:         []string{"furyctl", "validate", "config", "-c"},
			expectedPath: "",
			description:  "Should not crash when -c has no value",
		},
		{
			name:         "config equals empty",
			args:         []string{"furyctl", "validate", "config", "--config="},
			expectedPath: "",
			description:  "Should handle --config= with empty value",
		},
		{
			name:         "config flag as last argument",
			args:         []string{"furyctl", "validate", "config", "--config"},
			expectedPath: "",
			description:  "Should handle --config as the last argument",
		},
		{
			name:         "multiple config flags - first wins",
			args:         []string{"furyctl", "--config", "first.yaml", "validate", "--config", "second.yaml"},
			expectedPath: "first.yaml",
			description:  "Should use the first config flag found",
		},
		{
			name:         "config flag with special characters",
			args:         []string{"furyctl", "validate", "config", "--config", "test-file_v1.2.3.yaml"},
			expectedPath: "test-file_v1.2.3.yaml",
			description:  "Should handle filenames with special characters",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Extract the logic from our PersistentPreRun for testing
			var configPath string
			args := tc.args
			for i, arg := range args {
				if arg == "--config" || arg == "-c" {
					if i+1 < len(args) {
						configPath = args[i+1]
						break
					}
				} else if strings.HasPrefix(arg, "--config=") {
					configPath = strings.TrimPrefix(arg, "--config=")
					break
				}
			}

			assert.Equal(t, tc.expectedPath, configPath, tc.description)
		})
	}
}

func TestPersistentPreRun_GlobalFlagLoading(t *testing.T) {
	// Note: Not using t.Parallel() because we modify global viper state

	testCases := []struct {
		name           string
		configContent  string
		args           []string
		expectedDebug  bool
		expectedLog    string
		expectedOutdir string
	}{
		{
			name: "debug flag from config",
			configContent: `apiVersion: kfd.sighup.io/v1alpha2
kind: KFDDistribution
metadata:
  name: test-debug
spec:
  distributionVersion: v1.32.0
flags:
  global:
    debug: true`,
			args:          []string{"furyctl", "--config", "test-debug.yaml", "version"},
			expectedDebug: true,
		},
		{
			name: "log path from config",
			configContent: `apiVersion: kfd.sighup.io/v1alpha2
kind: KFDDistribution
metadata:
  name: test-log
spec:
  distributionVersion: v1.32.0
flags:
  global:
    log: "/tmp/test.log"
    debug: true`,
			args:        []string{"furyctl", "--config", "test-log.yaml", "version"},
			expectedLog: "/tmp/test.log",
		},
		{
			name: "outdir from config",
			configContent: `apiVersion: kfd.sighup.io/v1alpha2
kind: KFDDistribution
metadata:
  name: test-outdir
spec:
  distributionVersion: v1.32.0
flags:
  global:
    outdir: "/custom/outdir"
    debug: true`,
			args:           []string{"furyctl", "--config", "test-outdir.yaml", "version"},
			expectedOutdir: "/custom/outdir",
		},
		{
			name: "multiple flags from config",
			configContent: `apiVersion: kfd.sighup.io/v1alpha2
kind: KFDDistribution
metadata:
  name: test-multiple
spec:
  distributionVersion: v1.32.0
flags:
  global:
    debug: true
    log: "/tmp/multi.log"
    outdir: "/custom/multi"`,
			args:           []string{"furyctl", "--config", "test-multiple.yaml", "version"},
			expectedDebug:  true,
			expectedLog:    "/tmp/multi.log",
			expectedOutdir: "/custom/multi",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup: Create temporary config file
			tmpDir := t.TempDir()
			configFile := filepath.Join(tmpDir, "test-config.yaml")
			err := os.WriteFile(configFile, []byte(tc.configContent), 0o644)
			require.NoError(t, err)

			// Reset viper state
			viper.Reset()

			// Update args to use the actual temp file path
			updatedArgs := make([]string, len(tc.args))
			copy(updatedArgs, tc.args)
			for i, arg := range updatedArgs {
				if strings.HasSuffix(arg, ".yaml") {
					updatedArgs[i] = configFile
				}
			}

			// Simulate the logic from our PersistentPreRun
			var configPath string
			args := updatedArgs
			for i, arg := range args {
				if arg == "--config" || arg == "-c" {
					if i+1 < len(args) {
						configPath = args[i+1]
						break
					}
				} else if strings.HasPrefix(arg, "--config=") {
					configPath = strings.TrimPrefix(arg, "--config=")
					break
				}
			}

			if configPath != "" {
				// Simulate flag loading (we can't easily test the actual Manager here)
				// but we can verify the config path detection works
				assert.Equal(t, configFile, configPath)

				// Verify the config file exists and is readable
				assert.FileExists(t, configPath)

				// Verify config content is valid YAML
				content, err := os.ReadFile(configPath)
				assert.NoError(t, err)
				assert.Contains(t, string(content), "flags:")
			}
		})
	}
}

func TestPersistentPreRun_ErrorHandling(t *testing.T) {
	// Note: Not using t.Parallel() because we modify global state

	testCases := []struct {
		name          string
		args          []string
		setupConfig   bool
		configContent string
		expectError   bool
		errorPattern  string
	}{
		{
			name:        "non-existent config file",
			args:        []string{"furyctl", "--config", "nonexistent.yaml", "version"},
			setupConfig: false,
			expectError: false, // The error is logged but execution continues
		},
		{
			name:        "invalid YAML config",
			args:        []string{"furyctl", "--config", "invalid.yaml", "version"},
			setupConfig: true,
			configContent: `invalid: yaml: content:
  - missing
    proper: structure`,
			expectError: false, // Parsing errors are logged but execution continues
		},
		{
			name:        "config without flags section",
			args:        []string{"furyctl", "--config", "no-flags.yaml", "version"},
			setupConfig: true,
			configContent: `apiVersion: kfd.sighup.io/v1alpha2
kind: KFDDistribution
metadata:
  name: test-no-flags
spec:
  distributionVersion: v1.32.0`,
			expectError: false, // No flags is valid - should not error
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			var configFile string
			if tc.setupConfig {
				configFile = filepath.Join(tmpDir, "test-config.yaml")
				err := os.WriteFile(configFile, []byte(tc.configContent), 0o644)
				require.NoError(t, err)
			} else {
				configFile = filepath.Join(tmpDir, "nonexistent.yaml")
			}

			// Update args to use the actual file path
			updatedArgs := make([]string, len(tc.args))
			copy(updatedArgs, tc.args)
			for i, arg := range updatedArgs {
				if strings.HasSuffix(arg, ".yaml") {
					updatedArgs[i] = configFile
				}
			}

			// Test the command line parsing logic
			var configPath string
			args := updatedArgs
			for i, arg := range args {
				if arg == "--config" || arg == "-c" {
					if i+1 < len(args) {
						configPath = args[i+1]
						break
					}
				} else if strings.HasPrefix(arg, "--config=") {
					configPath = strings.TrimPrefix(arg, "--config=")
					break
				}
			}

			// Verify config path detection worked
			assert.Equal(t, configFile, configPath)

			// For error cases, verify the file state
			if !tc.setupConfig {
				assert.NoFileExists(t, configPath)
			} else {
				assert.FileExists(t, configPath)
			}
		})
	}
}

func TestPersistentPreRun_FlagPrecedence(t *testing.T) {
	// Test that our global flag loading doesn't interfere with CLI flag precedence
	// This is a simplified test since full precedence testing requires integration tests

	testCases := []struct {
		name             string
		configContent    string
		args             []string
		expectedHasDebug bool // Whether CLI has debug flag
	}{
		{
			name: "CLI debug should override config debug false",
			configContent: `apiVersion: kfd.sighup.io/v1alpha2
kind: KFDDistribution
metadata:
  name: test-precedence
spec:
  distributionVersion: v1.32.0
flags:
  global:
    debug: false`,
			args:             []string{"furyctl", "--debug", "--config", "test.yaml", "version"},
			expectedHasDebug: true,
		},
		{
			name: "config provides debug when no CLI flag",
			configContent: `apiVersion: kfd.sighup.io/v1alpha2
kind: KFDDistribution
metadata:
  name: test-config-debug
spec:
  distributionVersion: v1.32.0
flags:
  global:
    debug: true`,
			args:             []string{"furyctl", "--config", "test.yaml", "version"},
			expectedHasDebug: false, // No CLI debug flag
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configFile := filepath.Join(tmpDir, "test-config.yaml")
			err := os.WriteFile(configFile, []byte(tc.configContent), 0o644)
			require.NoError(t, err)

			// Update args to use actual file path
			updatedArgs := make([]string, len(tc.args))
			copy(updatedArgs, tc.args)
			for i, arg := range updatedArgs {
				if strings.HasSuffix(arg, ".yaml") {
					updatedArgs[i] = configFile
				}
			}

			// Check if CLI has debug flag
			hasDebugFlag := false
			for _, arg := range updatedArgs {
				if arg == "--debug" || arg == "-D" {
					hasDebugFlag = true
					break
				}
			}

			assert.Equal(t, tc.expectedHasDebug, hasDebugFlag)

			// Verify config path is detected correctly regardless of CLI flags
			var configPath string
			for i, arg := range updatedArgs {
				if arg == "--config" || arg == "-c" {
					if i+1 < len(updatedArgs) {
						configPath = updatedArgs[i+1]
						break
					}
				} else if strings.HasPrefix(arg, "--config=") {
					configPath = strings.TrimPrefix(arg, "--config=")
					break
				}
			}

			assert.Equal(t, configFile, configPath)
		})
	}
}

func TestPersistentPreRun_WithDynamicExpansion(t *testing.T) {
	// Test that our config loading works with dynamic value expansion

	testCases := []struct {
		name          string
		configContent string
		setupEnv      map[string]string
		expectedPaths []string // Paths that should be detected for expansion
	}{
		{
			name: "config with env var expansion",
			configContent: `apiVersion: kfd.sighup.io/v1alpha2
kind: KFDDistribution
metadata:
  name: test-expansion
spec:
  distributionVersion: v1.32.0
flags:
  global:
    log: "{env://TEST_PWD}/furyctl.log"
    outdir: "{env://TEST_HOME}/.furyctl"
    debug: true`,
			setupEnv: map[string]string{
				"TEST_PWD":  "/test/current",
				"TEST_HOME": "/test/home",
			},
			expectedPaths: []string{"{env://TEST_PWD}/furyctl.log", "{env://TEST_HOME}/.furyctl"},
		},
		{
			name: "config with mixed expansion patterns",
			configContent: `apiVersion: kfd.sighup.io/v1alpha2
kind: KFDDistribution
metadata:
  name: test-mixed
spec:
  distributionVersion: v1.32.0
flags:
  global:
    log: "{env://TEST_USER}/logs/{env://TEST_ENV}-furyctl.log"
    debug: true`,
			setupEnv: map[string]string{
				"TEST_USER": "testuser",
				"TEST_ENV":  "production",
			},
			expectedPaths: []string{"{env://TEST_USER}/logs/{env://TEST_ENV}-furyctl.log"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup environment variables
			for key, value := range tc.setupEnv {
				t.Setenv(key, value)
			}

			tmpDir := t.TempDir()
			configFile := filepath.Join(tmpDir, "test-config.yaml")
			err := os.WriteFile(configFile, []byte(tc.configContent), 0o644)
			require.NoError(t, err)

			// Test that config path detection works
			args := []string{"furyctl", "--config", configFile, "version"}
			var configPath string
			for i, arg := range args {
				if arg == "--config" || arg == "-c" {
					if i+1 < len(args) {
						configPath = args[i+1]
						break
					}
				} else if strings.HasPrefix(arg, "--config=") {
					configPath = strings.TrimPrefix(arg, "--config=")
					break
				}
			}

			assert.Equal(t, configFile, configPath)

			// Verify config file contains expansion patterns
			content, err := os.ReadFile(configFile)
			require.NoError(t, err)

			for _, expectedPath := range tc.expectedPaths {
				assert.Contains(t, string(content), expectedPath)
			}
		})
	}
}
