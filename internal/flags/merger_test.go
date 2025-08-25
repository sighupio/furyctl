// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package flags_test

import (
	"testing"

	"github.com/spf13/viper"

	"github.com/sighupio/furyctl/internal/flags"
)

func TestMerger_MergeIntoViper(t *testing.T) {
	tests := []struct {
		name           string
		flags          *flags.FlagsConfig
		command        string
		expectedValues map[string]any
		setupViper     func()
	}{
		{
			name: "merge global and apply flags",
			flags: &flags.FlagsConfig{
				Global: map[string]any{
					"debug":            true,
					"disableAnalytics": false,
				},
				Apply: map[string]any{
					"skipDepsValidation": true,
					"dryRun":             false,
					"timeout":            7200,
				},
			},
			command: "apply",
			expectedValues: map[string]any{
				"debug":                true,
				"disable-analytics":    false,
				"skip-deps-validation": true,
				"dry-run":              false,
				"timeout":              7200,
			},
			setupViper: func() {
				viper.Reset()
			},
		},
		{
			name: "flags do not override existing viper values",
			flags: &flags.FlagsConfig{
				Global: map[string]any{
					"debug": true,
				},
				Apply: map[string]any{
					"dryRun": true,
				},
			},
			command: "apply",
			expectedValues: map[string]any{
				"debug":   false, // Should remain false (already set in viper)
				"dry-run": true,  // Should be set from config (not in viper)
			},
			setupViper: func() {
				viper.Reset()
				viper.Set("debug", false) // Pre-set this value
			},
		},
		{
			name: "merge only global flags for unknown command",
			flags: &flags.FlagsConfig{
				Global: map[string]any{
					"debug": true,
				},
				Apply: map[string]any{
					"dryRun": true,
				},
			},
			command: "unknown",
			expectedValues: map[string]any{
				"debug":   true,
				"dry-run": nil, // Should not be set
			},
			setupViper: func() {
				viper.Reset()
			},
		},
		{
			name:           "nil flags",
			flags:          nil,
			command:        "apply",
			expectedValues: map[string]any{},
			setupViper: func() {
				viper.Reset()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupViper()

			merger := flags.NewMerger()
			err := merger.MergeIntoViper(tt.flags, tt.command)
			if err != nil {
				t.Fatalf("MergeIntoViper() error = %v", err)
			}

			// Check expected values
			for key, expectedValue := range tt.expectedValues {
				actualValue := viper.Get(key)

				if expectedValue == nil {
					if actualValue != nil {
						t.Errorf("Expected %s to be nil, got: %v", key, actualValue)
					}
				} else {
					if actualValue != expectedValue {
						t.Errorf("Expected %s to be %v, got: %v", key, expectedValue, actualValue)
					}
				}
			}
		})
	}
}

func TestCamelToKebab(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"distroLocation", "distro-location"},
		{"skipDepsDownload", "skip-deps-download"},
		{"skipDepsValidation", "skip-deps-validation"},
		{"binPath", "bin-path"},
		{"dryRun", "dry-run"},
		{"vpnAutoConnect", "vpn-auto-connect"},
		{"skipVpnConfirmation", "skip-vpn-confirmation"},
		{"disableAnalytics", "disable-analytics"},
		{"gitProtocol", "git-protocol"},
		{"skipNodesUpgrade", "skip-nodes-upgrade"},
		{"podRunningCheckTimeout", "pod-running-check-timeout"},
		{"upgradePathLocation", "upgrade-path-location"},
		{"upgradeNode", "upgrade-node"},
		{"postApplyPhases", "post-apply-phases"},
		{"noTty", "no-tty"},
		{"startFrom", "start-from"},
		{"autoApprove", "auto-approve"},
		// Edge cases
		{"debug", "debug"},     // No camelCase
		{"timeout", "timeout"}, // No camelCase
		{"force", "force"},     // No camelCase
		{"workdir", "workdir"}, // No camelCase
		{"outdir", "outdir"},   // No camelCase
		{"log", "log"},         // No camelCase
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := flags.CamelToKebab(tt.input)
			if result != tt.expected {
				t.Errorf("camelToKebab(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMerger_ConvertValue(t *testing.T) {
	merger := flags.NewMerger()

	tests := []struct {
		name         string
		value        any
		expectedType flags.FlagType
		expected     any
		expectError  bool
	}{
		{
			name:         "bool true",
			value:        true,
			expectedType: flags.FlagTypeBool,
			expected:     true,
			expectError:  false,
		},
		{
			name:         "bool from string true",
			value:        "true",
			expectedType: flags.FlagTypeBool,
			expected:     true,
			expectError:  false,
		},
		{
			name:         "bool from string false",
			value:        "false",
			expectedType: flags.FlagTypeBool,
			expected:     false,
			expectError:  false,
		},
		{
			name:         "int from int",
			value:        42,
			expectedType: flags.FlagTypeInt,
			expected:     42,
			expectError:  false,
		},
		{
			name:         "int from float64",
			value:        42.0,
			expectedType: flags.FlagTypeInt,
			expected:     42,
			expectError:  false,
		},
		{
			name:         "int from string",
			value:        "42",
			expectedType: flags.FlagTypeInt,
			expected:     42,
			expectError:  false,
		},
		{
			name:         "string from any",
			value:        123,
			expectedType: flags.FlagTypeString,
			expected:     "123",
			expectError:  false,
		},
		{
			name:         "string slice from array",
			value:        []any{"a", "b", "c"},
			expectedType: flags.FlagTypeStringSlice,
			expected:     []string{"a", "b", "c"},
			expectError:  false,
		},
		{
			name:         "string slice from comma separated string",
			value:        "a,b,c",
			expectedType: flags.FlagTypeStringSlice,
			expected:     []string{"a", "b", "c"},
			expectError:  false,
		},
		{
			name:         "string slice from empty string",
			value:        "",
			expectedType: flags.FlagTypeStringSlice,
			expected:     []string{},
			expectError:  false,
		},
		{
			name:         "invalid bool",
			value:        "invalid",
			expectedType: flags.FlagTypeBool,
			expected:     nil,
			expectError:  true,
		},
		{
			name:         "invalid int",
			value:        "not-a-number",
			expectedType: flags.FlagTypeInt,
			expected:     nil,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the ConvertValue method directly
			actualValue, err := merger.ConvertValue(tt.value, tt.expectedType)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				// For string slices, we need to compare them properly
				if expectedSlice, ok := tt.expected.([]string); ok {
					if actualSlice, ok := actualValue.([]string); ok {
						if len(expectedSlice) != len(actualSlice) {
							t.Errorf("Expected %v, got %v", tt.expected, actualValue)
						} else {
							for i := range expectedSlice {
								if expectedSlice[i] != actualSlice[i] {
									t.Errorf("Expected %v, got %v", tt.expected, actualValue)
									break
								}
							}
						}
					} else {
						t.Errorf("Expected []string type, got %T", actualValue)
					}
				} else if actualValue != tt.expected {
					t.Errorf("Expected %v, got %v", tt.expected, actualValue)
				}
			}
		})
	}
}

func TestMerger_MergeGlobalFlags(t *testing.T) {
	defer viper.Reset()

	flagsConfig := &flags.FlagsConfig{
		Global: map[string]any{
			"debug":            true,
			"disableAnalytics": false,
		},
	}

	merger := flags.NewMerger()
	err := merger.MergeGlobalFlags(flagsConfig)
	if err != nil {
		t.Fatalf("MergeGlobalFlags() error = %v", err)
	}

	if viper.GetBool("debug") != true {
		t.Errorf("Expected debug to be true, got: %v", viper.GetBool("debug"))
	}

	if viper.GetBool("disableAnalytics") != false {
		t.Errorf("Expected disableAnalytics to be false, got: %v", viper.GetBool("disableAnalytics"))
	}
}

func TestMerger_GetSupportedFlagsForCommand(t *testing.T) {
	merger := flags.NewMerger()

	tests := []struct {
		command   string
		expectNil bool
	}{
		{"global", false},
		{"apply", false},
		{"delete", false},
		{"create", false},
		{"unknown", true},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			supportedFlags := merger.GetSupportedFlagsForCommand(tt.command)

			if tt.expectNil && supportedFlags != nil {
				t.Errorf("Expected nil for unknown command, got: %+v", supportedFlags)
			}

			if !tt.expectNil && supportedFlags == nil {
				t.Errorf("Expected supported flags for command %s, got nil", tt.command)
			}
		})
	}
}
