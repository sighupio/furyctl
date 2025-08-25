// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package parser_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/parser"
)

func TestDynamicValueExpansion_EnvVars(t *testing.T) {
	testCases := []struct {
		name         string
		envVar       string
		envValue     string
		inputValue   string
		expected     string
		shouldError  bool
		errorPattern string
	}{
		{
			name:       "expand single env var - PWD",
			envVar:     "TEST_PWD",
			envValue:   "/test/current/dir",
			inputValue: "{env://TEST_PWD}",
			expected:   "/test/current/dir",
		},
		{
			name:       "expand single env var - HOME",
			envVar:     "TEST_HOME",
			envValue:   "/home/testuser",
			inputValue: "{env://TEST_HOME}",
			expected:   "/home/testuser",
		},
		{
			name:       "expand env var in path - log file",
			envVar:     "TEST_PWD",
			envValue:   "/test/current/dir",
			inputValue: "{env://TEST_PWD}/furyctl.log",
			expected:   "/test/current/dir/furyctl.log",
		},
		{
			name:       "expand env var in path - outdir",
			envVar:     "TEST_HOME",
			envValue:   "/home/testuser",
			inputValue: "{env://TEST_HOME}/.furyctl",
			expected:   "/home/testuser/.furyctl",
		},
		{
			name:       "expand multiple env vars",
			envVar:     "TEST_USER",
			envValue:   "testuser",
			inputValue: "{env://TEST_PWD}/logs/{env://TEST_USER}-furyctl.log",
			expected:   "/test/current/dir/logs/testuser-furyctl.log",
		},
		{
			name:         "missing environment variable",
			envVar:       "", // Don't set env var
			inputValue:   "{env://NONEXISTENT_VAR}",
			shouldError:  true,
			errorPattern: "NONEXISTENT_VAR\" is empty",
		},
		{
			name:         "empty environment variable",
			envVar:       "TEST_EMPTY",
			envValue:     "",
			inputValue:   "{env://TEST_EMPTY}",
			shouldError:  true,
			errorPattern: "TEST_EMPTY\" is empty",
		},
		{
			name:       "env var with newline trimmed",
			envVar:     "TEST_NEWLINE",
			envValue:   "/test/path\n",
			inputValue: "{env://TEST_NEWLINE}",
			expected:   "/test/path",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup environment variable if specified
			if tc.envVar != "" {
				t.Setenv(tc.envVar, tc.envValue)
			}

			// For multiple env vars test, also set TEST_PWD
			if tc.name == "expand multiple env vars" {
				t.Setenv("TEST_PWD", "/test/current/dir")
			}

			parser := parser.NewConfigParser(".")

			result, err := parser.ParseDynamicValue(tc.inputValue)

			if tc.shouldError {
				assert.Error(t, err)
				if tc.errorPattern != "" {
					assert.Contains(t, err.Error(), tc.errorPattern)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestDynamicValueExpansion_FilePaths(t *testing.T) {
	testCases := []struct {
		name         string
		setupFile    func() (string, func()) // Returns file path and cleanup function
		inputValue   string
		expected     string
		shouldError  bool
		errorPattern string
	}{
		{
			name: "expand file content - absolute path",
			setupFile: func() (string, func()) {
				tmpDir := t.TempDir()
				filePath := filepath.Join(tmpDir, "test-config.txt")
				err := os.WriteFile(filePath, []byte("/custom/log/path"), 0o644)
				require.NoError(t, err)
				return filePath, func() {} // TempDir handles cleanup
			},
			inputValue: "", // Will be set by setupFile
			expected:   "/custom/log/path",
		},
		{
			name: "expand file content - relative path",
			setupFile: func() (string, func()) {
				tmpDir := t.TempDir()
				filePath := filepath.Join(tmpDir, "relative-config.txt")
				err := os.WriteFile(filePath, []byte("debug-enabled"), 0o644)
				require.NoError(t, err)
				// Change to tmpDir so relative path works
				oldWd, err := os.Getwd()
				require.NoError(t, err)
				err = os.Chdir(tmpDir)
				require.NoError(t, err)
				return "./relative-config.txt", func() {
					os.Chdir(oldWd)
				}
			},
			inputValue: "", // Will be set by setupFile
			expected:   "debug-enabled",
		},
		{
			name: "expand file content with newline trimmed",
			setupFile: func() (string, func()) {
				tmpDir := t.TempDir()
				filePath := filepath.Join(tmpDir, "newline-config.txt")
				err := os.WriteFile(filePath, []byte("/path/with/newline\n"), 0o644)
				require.NoError(t, err)
				return filePath, func() {}
			},
			inputValue: "", // Will be set by setupFile
			expected:   "/path/with/newline",
		},
		{
			name:         "missing file",
			setupFile:    nil,
			inputValue:   "{file://./nonexistent-file.txt}",
			shouldError:  true,
			errorPattern: "no such file or directory",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var inputValue string
			var cleanup func()

			if tc.setupFile != nil {
				filePath, cleanupFunc := tc.setupFile()
				inputValue = "{file://" + filePath + "}"
				cleanup = cleanupFunc
				defer cleanup()
			} else {
				inputValue = tc.inputValue
			}

			parser := parser.NewConfigParser(".")

			result, err := parser.ParseDynamicValue(inputValue)

			if tc.shouldError {
				assert.Error(t, err)
				if tc.errorPattern != "" {
					assert.Contains(t, err.Error(), tc.errorPattern)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestDynamicValueExpansion_MixedPatterns(t *testing.T) {
	testCases := []struct {
		name        string
		setup       func() func() // Returns cleanup function
		inputValue  string
		expected    string
		shouldError bool
	}{
		{
			name: "mixed env and path - typical log path",
			setup: func() func() {
				t.Setenv("TEST_PWD", "/current/directory")
				t.Setenv("TEST_USER", "testuser")
				return func() {}
			},
			inputValue: "{env://TEST_PWD}/logs/{env://TEST_USER}-furyctl.log",
			expected:   "/current/directory/logs/testuser-furyctl.log",
		},
		{
			name: "mixed env vars - complex outdir",
			setup: func() func() {
				t.Setenv("TEST_HOME", "/home/testuser")
				t.Setenv("TEST_PROJECT", "myproject")
				return func() {}
			},
			inputValue: "{env://TEST_HOME}/projects/{env://TEST_PROJECT}/.furyctl",
			expected:   "/home/testuser/projects/myproject/.furyctl",
		},
		{
			name: "prefix and suffix with env var",
			setup: func() func() {
				t.Setenv("TEST_ENV", "production")
				return func() {}
			},
			inputValue: "prefix-{env://TEST_ENV}-suffix",
			expected:   "prefix-production-suffix",
		},
		{
			name: "multiple same env var",
			setup: func() func() {
				t.Setenv("TEST_VAR", "repeated")
				return func() {}
			},
			inputValue: "{env://TEST_VAR}/path/{env://TEST_VAR}",
			expected:   "repeated/path/repeated",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cleanup := tc.setup()
			defer cleanup()

			parser := parser.NewConfigParser(".")

			result, err := parser.ParseDynamicValue(tc.inputValue)

			if tc.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestDynamicValueExpansion_ErrorHandling(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		inputValue   string
		shouldError  bool
		errorPattern string
	}{
		{
			name:        "invalid syntax - missing closing brace",
			inputValue:  "{env://TEST_VAR",
			shouldError: false, // This doesn't match the pattern, so returns as-is
		},
		{
			name:        "invalid syntax - missing opening brace",
			inputValue:  "env://TEST_VAR}",
			shouldError: false, // This doesn't match the pattern, so returns as-is
		},
		{
			name:         "empty env var name",
			inputValue:   "{env://}",
			shouldError:  true,
			errorPattern: "is empty",
		},
		{
			name:        "unknown expansion type",
			inputValue:  "{unknown://test}",
			shouldError: false, // Unknown types return as-is
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parser := parser.NewConfigParser(".")

			result, err := parser.ParseDynamicValue(tc.inputValue)

			if tc.shouldError {
				assert.Error(t, err)
				if tc.errorPattern != "" {
					assert.Contains(t, err.Error(), tc.errorPattern)
				}
			} else {
				assert.NoError(t, err)
				// For invalid syntax that doesn't error, verify it returns unchanged
				if strings.Contains(tc.name, "invalid syntax") || strings.Contains(tc.name, "unknown") {
					assert.Equal(t, tc.inputValue, result)
				}
			}
		})
	}
}

func TestDynamicValueExpansion_FlagsSpecificScenarios(t *testing.T) {
	// Test scenarios specific to flags feature that were fixed in Issue #1-5
	testCases := []struct {
		name       string
		setup      func() func()
		inputValue string
		expected   string
		testType   string // "log", "debug", "outdir"
	}{
		{
			name: "flags.global.log with PWD expansion",
			setup: func() func() {
				t.Setenv("PWD", "/test/working/dir")
				return func() {}
			},
			inputValue: "{env://PWD}/furyctl.log",
			expected:   "/test/working/dir/furyctl.log",
			testType:   "log",
		},
		{
			name: "flags.global.outdir with PWD expansion",
			setup: func() func() {
				t.Setenv("PWD", "/test/working/dir")
				return func() {}
			},
			inputValue: "{env://PWD}",
			expected:   "/test/working/dir",
			testType:   "outdir",
		},
		{
			name: "flags.global.log with HOME expansion",
			setup: func() func() {
				t.Setenv("HOME", "/home/testuser")
				return func() {}
			},
			inputValue: "{env://HOME}/.furyctl/furyctl.log",
			expected:   "/home/testuser/.furyctl/furyctl.log",
			testType:   "log",
		},
		{
			name: "complex log path - multiple env vars",
			setup: func() func() {
				t.Setenv("PWD", "/current/dir")
				t.Setenv("USER", "testuser")
				return func() {}
			},
			inputValue: "{env://PWD}/logs/{env://USER}-furyctl.log",
			expected:   "/current/dir/logs/testuser-furyctl.log",
			testType:   "log",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cleanup := tc.setup()
			defer cleanup()

			parser := parser.NewConfigParser(".")

			result, err := parser.ParseDynamicValue(tc.inputValue)

			assert.NoError(t, err)
			assert.Equal(t, tc.expected, result)

			// Additional verification based on test type
			switch tc.testType {
			case "log":
				// Verify it looks like a valid log path
				assert.True(t, strings.HasSuffix(result.(string), ".log") || result == "stdout")
			case "outdir":
				// Verify it looks like a valid directory path
				assert.True(t, strings.HasPrefix(result.(string), "/") || strings.HasPrefix(result.(string), "."))
			}
		})
	}
}

func TestDynamicValueExpansion_NonStringTypes(t *testing.T) {
	// Test that non-string types are handled correctly
	testCases := []struct {
		name        string
		inputValue  any
		expected    any
		shouldError bool
	}{
		{
			name:       "boolean value",
			inputValue: true,
			expected:   true,
		},
		{
			name:       "integer value",
			inputValue: 42,
			expected:   42,
		},
		{
			name:       "float value",
			inputValue: 3.14,
			expected:   3.14,
		},
		{
			name:       "nil value",
			inputValue: nil,
			expected:   nil,
		},
		{
			name:       "string slice with expansion",
			inputValue: []string{"{env://TEST_VAR}", "static"},
			expected:   []any{"test-value", "static"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup env var for slice test
			if tc.name == "string slice with expansion" {
				t.Setenv("TEST_VAR", "test-value")
			}

			parser := parser.NewConfigParser(".")

			result, err := parser.ParseDynamicValue(tc.inputValue)

			if tc.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}
