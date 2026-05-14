// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package flags_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/flags"
)

func TestLoader_LoadFromFile(t *testing.T) {
	tests := []struct {
		name           string
		configContent  string
		expectedFlags  *flags.FlagsConfig
		expectedErrors int
	}{
		{
			name: "valid flags configuration",
			configContent: `apiVersion: kfd.sighup.io/v1alpha2
kind: OnPremises
metadata:
  name: test-cluster
spec:
  distributionVersion: v1.31.0
flags:
  global:
    debug: true
    disableAnalytics: false
  apply:
    skipDepsValidation: true
    distroLocation: "/tmp/test"
    dryRun: false`,
			expectedFlags: &flags.FlagsConfig{
				Global: map[string]any{
					"debug":            true,
					"disableAnalytics": false,
				},
				Apply: map[string]any{
					"skipDepsValidation": true,
					"distroLocation":     "/tmp/test",
					"dryRun":             false,
				},
			},
			expectedErrors: 0,
		},
		{
			name: "no flags section",
			configContent: `apiVersion: kfd.sighup.io/v1alpha2
kind: OnPremises
metadata:
  name: test-cluster
spec:
  distributionVersion: v1.31.0`,
			expectedFlags:  nil,
			expectedErrors: 0,
		},
		{
			name: "flags with dynamic values",
			configContent: `apiVersion: kfd.sighup.io/v1alpha2
kind: OnPremises
metadata:
  name: test-cluster
spec:
  distributionVersion: v1.31.0
flags:
  global:
    outdir: "{env://TEST_OUTDIR}"
  apply:
    distroPatches: "{env://TEST_PATCHES}"`,
			expectedFlags: &flags.FlagsConfig{
				Global: map[string]any{
					"outdir": "/test/output",
				},
				Apply: map[string]any{
					"distroPatches": "/test/patches",
				},
			},
			expectedErrors: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set test environment variables if needed
			if tt.name == "flags with dynamic values" {
				t.Setenv("TEST_OUTDIR", "/test/output")
				t.Setenv("TEST_PATCHES", "/test/patches")
			}

			// Create temporary config file
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "furyctl.yaml")

			err := os.WriteFile(configPath, []byte(tt.configContent), 0o644)
			if err != nil {
				t.Fatalf("Failed to create test config file: %v", err)
			}

			// Create loader and load flags
			loader := flags.NewLoader(tmpDir)
			result, err := loader.LoadFromFile(configPath)
			if err != nil {
				t.Fatalf("LoadFromFile() error = %v", err)
			}

			// Check errors count
			if len(result.Errors) != tt.expectedErrors {
				t.Errorf("Expected %d errors, got %d: %v", tt.expectedErrors, len(result.Errors), result.Errors)
			}

			// Check flags result
			if tt.expectedFlags == nil && result.Flags != nil {
				t.Errorf("Expected no flags, but got: %+v", result.Flags)
			}

			if tt.expectedFlags != nil && result.Flags == nil {
				t.Errorf("Expected flags but got nil")
			}

			if tt.expectedFlags != nil && result.Flags != nil {
				// Compare global flags
				assert.Equal(t, tt.expectedFlags.Global, result.Flags.Global)
				// Compare apply flags
				assert.Equal(t, tt.expectedFlags.Apply, result.Flags.Apply)
			}
		})
	}
}

func TestLoader_LoadFromFile_NonExistentFile(t *testing.T) {
	loader := flags.NewLoader(".")
	result, err := loader.LoadFromFile("/nonexistent/path/furyctl.yaml")
	require.NoError(t, err)

	assert.NotEmpty(t, result.Errors)
	assert.Nil(t, result.Flags)
}

func TestLoader_LoadFromDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a furyctl.yaml file
	configContent := `apiVersion: kfd.sighup.io/v1alpha2
kind: OnPremises
metadata:
  name: test-cluster
spec:
  distributionVersion: v1.31.0
flags:
  global:
    debug: true`

	configPath := filepath.Join(tmpDir, "furyctl.yaml")
	err := os.WriteFile(configPath, []byte(configContent), 0o644)
	require.NoError(t, err)

	// Test loading from directory
	loader := flags.NewLoader(tmpDir)
	result, err := loader.LoadFromDirectory(tmpDir)
	require.NoError(t, err)

	assert.Empty(t, result.Errors)
	require.NotNil(t, result.Flags)
	require.NotNil(t, result.Flags.Global)

	debugValue := result.Flags.Global["debug"]
	// The value might be parsed as a string "true" or boolean true depending on YAML parser
	assert.Contains(t, []any{true, "true"}, debugValue)
}

func TestLoader_LoadFromDirectory_NoConfigFile(t *testing.T) {
	tmpDir := t.TempDir()

	loader := flags.NewLoader(tmpDir)
	result, err := loader.LoadFromDirectory(tmpDir)
	require.NoError(t, err)

	assert.NotEmpty(t, result.Errors)
	assert.Nil(t, result.Flags)
}
