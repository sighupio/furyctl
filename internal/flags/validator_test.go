// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package flags_test

import (
	"testing"

	"github.com/sighupio/furyctl/internal/flags"
)

func TestValidator_Validate(t *testing.T) {
	validator := flags.NewValidator()

	tests := []struct {
		name           string
		flags          *flags.FlagsConfig
		expectedErrors int
	}{
		{
			name: "valid flags configuration",
			flags: &flags.FlagsConfig{
				Global: map[string]any{
					"debug":            true,
					"disableAnalytics": false,
					"gitProtocol":      "https",
				},
				Apply: map[string]any{
					"skipDepsValidation": true,
					"dryRun":             false,
					"timeout":            3600,
					"force":              []any{"all"},
				},
			},
			expectedErrors: 0,
		},
		{
			name: "unsupported flags",
			flags: &flags.FlagsConfig{
				Global: map[string]any{
					"unknownFlag": "value",
				},
				Apply: map[string]any{
					"anotherUnknownFlag": true,
				},
			},
			expectedErrors: 2, // Two unsupported flags
		},
		{
			name: "invalid git protocol",
			flags: &flags.FlagsConfig{
				Global: map[string]any{
					"gitProtocol": "invalid",
				},
			},
			expectedErrors: 1,
		},
		{
			name: "invalid force options",
			flags: &flags.FlagsConfig{
				Apply: map[string]any{
					"force": []any{"invalid-option"},
				},
			},
			expectedErrors: 1,
		},
		{
			name: "invalid timeout",
			flags: &flags.FlagsConfig{
				Apply: map[string]any{
					"timeout": -1,
				},
			},
			expectedErrors: 1,
		},
		{
			name: "conflicting vpn flags",
			flags: &flags.FlagsConfig{
				Apply: map[string]any{
					"skipVpnConfirmation": true,
					"vpnAutoConnect":      true,
				},
			},
			expectedErrors: 1, // Conflicting flags
		},
		{
			name: "conflicting upgrade flags",
			flags: &flags.FlagsConfig{
				Apply: map[string]any{
					"upgrade":     true,
					"upgradeNode": "worker1",
				},
			},
			expectedErrors: 1, // Conflicting flags
		},
		{
			name: "conflicting phase and startFrom",
			flags: &flags.FlagsConfig{
				Apply: map[string]any{
					"phase":     "distribution",
					"startFrom": "infrastructure",
				},
			},
			expectedErrors: 1, // Conflicting flags
		},
		{
			name: "conflicting phase and postApplyPhases",
			flags: &flags.FlagsConfig{
				Apply: map[string]any{
					"phase":           "distribution",
					"postApplyPhases": []any{"distribution"},
				},
			},
			expectedErrors: 1, // Conflicting flags
		},
		{
			name:           "nil flags",
			flags:          nil,
			expectedErrors: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validator.Validate(tt.flags)

			if len(errors) != tt.expectedErrors {
				t.Errorf("Expected %d errors, got %d: %v", tt.expectedErrors, len(errors), errors)
			}
		})
	}
}

func TestValidator_ValidateSpecificFlag(t *testing.T) {
	validator := flags.NewValidator()

	tests := []struct {
		name        string
		flagName    string
		value       any
		expectError bool
	}{
		{
			name:        "valid gitProtocol https",
			flagName:    "gitProtocol",
			value:       "https",
			expectError: false,
		},
		{
			name:        "valid gitProtocol ssh",
			flagName:    "gitProtocol",
			value:       "ssh",
			expectError: false,
		},
		{
			name:        "invalid gitProtocol",
			flagName:    "gitProtocol",
			value:       "ftp",
			expectError: true,
		},
		{
			name:        "valid timeout",
			flagName:    "timeout",
			value:       3600,
			expectError: false,
		},
		{
			name:        "invalid timeout negative",
			flagName:    "timeout",
			value:       -1,
			expectError: true,
		},
		{
			name:        "valid force options",
			flagName:    "force",
			value:       []any{"all", "upgrades"},
			expectError: false,
		},
		{
			name:        "invalid force option",
			flagName:    "force",
			value:       []any{"invalid"},
			expectError: true,
		},
		{
			name:        "unknown flag",
			flagName:    "unknownFlag",
			value:       "value",
			expectError: false, // Should not error for unknown flags
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a simple flag info for testing
			flagInfo := flags.FlagInfo{
				Type:        flags.FlagTypeString,
				Description: "Test flag",
			}

			// Test through validateFlagValue which calls validateSpecificFlag
			err := validator.ValidateIndividualFlag(tt.flagName, tt.value, flagInfo)

			if tt.expectError && err == nil {
				t.Errorf("Expected error for flag %s with value %v, but got none", tt.flagName, tt.value)
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for flag %s with value %v: %v", tt.flagName, tt.value, err)
			}
		})
	}
}

func TestValidationError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      flags.ValidationError
		expected string
	}{
		{
			name: "fatal error with flag",
			err: flags.ValidationError{
				Command:  "apply",
				Flag:     "timeout",
				Value:    -1,
				Reason:   "must be positive",
				Severity: flags.ValidationSeverityFatal,
			},
			expected: "fatal validation error for apply.timeout: must be positive",
		},
		{
			name: "warning error with flag",
			err: flags.ValidationError{
				Command:  "global",
				Flag:     "unknownFlag",
				Value:    "value",
				Reason:   "unsupported flag",
				Severity: flags.ValidationSeverityWarning,
			},
			expected: "warning validation error for global.unknownFlag: unsupported flag",
		},
		{
			name: "error without specific flag",
			err: flags.ValidationError{
				Command:  "apply",
				Flag:     "",
				Value:    nil,
				Reason:   "unsupported command",
				Severity: flags.ValidationSeverityFatal,
			},
			expected: "fatal validation error for apply: unsupported command",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.expected {
				t.Errorf("Expected error message %q, got %q", tt.expected, tt.err.Error())
			}
		})
	}
}

func TestValidator_ErrorSeverityClassification(t *testing.T) {
	validator := flags.NewValidator()

	tests := []struct {
		name             string
		flags            *flags.FlagsConfig
		expectedFatal    int
		expectedWarnings int
	}{
		{
			name: "fatal errors - invalid protocol and negative timeout",
			flags: &flags.FlagsConfig{
				Global: map[string]any{
					"gitProtocol": "ftp", // Fatal: invalid protocol
				},
				Apply: map[string]any{
					"timeout": -5, // Fatal: negative timeout
				},
			},
			expectedFatal:    2,
			expectedWarnings: 0,
		},
		{
			name: "fatal errors - unsupported flags",
			flags: &flags.FlagsConfig{
				Global: map[string]any{
					"unknownGlobalFlag": "value", // Fatal: unsupported
				},
				Apply: map[string]any{
					"unknownApplyFlag": true, // Fatal: unsupported
				},
			},
			expectedFatal:    2,
			expectedWarnings: 0,
		},
		{
			name: "fatal errors - conflicting flags",
			flags: &flags.FlagsConfig{
				Apply: map[string]any{
					"vpnAutoConnect":      true, // Fatal: conflicts with skipVpnConfirmation
					"skipVpnConfirmation": true,
					"upgrade":             true, // Fatal: conflicts with upgradeNode
					"upgradeNode":         "worker1",
				},
			},
			expectedFatal:    2,
			expectedWarnings: 0,
		},
		{
			name: "fatal errors - invalid force options",
			flags: &flags.FlagsConfig{
				Apply: map[string]any{
					"force": []any{"invalid-option"}, // Fatal: invalid force option
				},
			},
			expectedFatal:    1,
			expectedWarnings: 0,
		},
		{
			name: "mixed fatal errors",
			flags: &flags.FlagsConfig{
				Global: map[string]any{
					"gitProtocol": "invalid", // Fatal: invalid protocol
					"unknownFlag": "value",   // Fatal: unsupported
				},
				Apply: map[string]any{
					"timeout":        -1,     // Fatal: negative timeout
					"anotherUnknown": "test", // Fatal: unsupported
				},
			},
			expectedFatal:    4,
			expectedWarnings: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validator.Validate(tt.flags)

			// Count fatal vs warning errors.
			fatalCount := 0
			warningCount := 0

			for _, err := range errors {
				if err.Severity == flags.ValidationSeverityFatal {
					fatalCount++
				} else if err.Severity == flags.ValidationSeverityWarning {
					warningCount++
				}
			}

			if fatalCount != tt.expectedFatal {
				t.Errorf("Expected %d fatal errors, got %d", tt.expectedFatal, fatalCount)
			}

			if warningCount != tt.expectedWarnings {
				t.Errorf("Expected %d warning errors, got %d", tt.expectedWarnings, warningCount)
			}
		})
	}
}
