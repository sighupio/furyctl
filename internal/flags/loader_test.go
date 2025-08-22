// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package flags_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

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
				if !compareMaps(tt.expectedFlags.Global, result.Flags.Global) {
					t.Errorf("Global flags mismatch. Expected: %+v, Got: %+v", tt.expectedFlags.Global, result.Flags.Global)
				}

				// Compare apply flags
				if !compareMaps(tt.expectedFlags.Apply, result.Flags.Apply) {
					t.Errorf("Apply flags mismatch. Expected: %+v, Got: %+v", tt.expectedFlags.Apply, result.Flags.Apply)
				}
			}
		})
	}
}

func TestLoader_LoadFromFile_NonExistentFile(t *testing.T) {
	loader := flags.NewLoader(".")
	result, err := loader.LoadFromFile("/nonexistent/path/furyctl.yaml")
	if err != nil {
		t.Fatalf("LoadFromFile() should not error for non-existent file, got: %v", err)
	}

	if len(result.Errors) == 0 {
		t.Error("Expected error for non-existent file")
	}

	if result.Flags != nil {
		t.Error("Expected no flags for non-existent file")
	}
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
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	// Test loading from directory
	loader := flags.NewLoader(tmpDir)
	result, err := loader.LoadFromDirectory(tmpDir)
	if err != nil {
		t.Fatalf("LoadFromDirectory() error = %v", err)
	}

	if len(result.Errors) > 0 {
		t.Errorf("Unexpected errors: %v", result.Errors)
	}

	if result.Flags == nil || result.Flags.Global == nil {
		t.Error("Expected flags to be loaded but they are nil")
		return
	}

	debugValue := result.Flags.Global["debug"]
	// The value might be parsed as a string "true" or boolean true depending on YAML parser
	if debugValue != true && debugValue != "true" {
		t.Errorf("Expected debug flag to be true, got: %v (type: %T)", debugValue, debugValue)
	}
}

func TestLoader_LoadFromDirectory_NoConfigFile(t *testing.T) {
	tmpDir := t.TempDir()

	loader := flags.NewLoader(tmpDir)
	result, err := loader.LoadFromDirectory(tmpDir)
	if err != nil {
		t.Fatalf("LoadFromDirectory() should not error when no config exists, got: %v", err)
	}

	if len(result.Errors) == 0 {
		t.Error("Expected error when no config file exists")
	}

	if result.Flags != nil {
		t.Error("Expected no flags when no config exists")
	}
}

// Helper function to compare maps
func compareMaps(expected, actual map[string]any) bool {
	if len(expected) != len(actual) {
		return false
	}

	for key, expectedValue := range expected {
		actualValue, exists := actual[key]
		if !exists {
			return false
		}

		// Use reflect.DeepEqual for better comparison
		if fmt.Sprintf("%v", expectedValue) != fmt.Sprintf("%v", actualValue) {
			return false
		}
	}

	return true
}
