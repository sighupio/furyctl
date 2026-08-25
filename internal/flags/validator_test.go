// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package flags_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sighupio/furyctl/internal/flags"
)

func TestValidator_Validate(t *testing.T) {
	validator := flags.NewValidator()

	tests := []struct {
		name           string
		flags          flags.FlagsConfig
		expectedErrors int
	}{
		{
			name: "valid flags configuration",
			flags: flags.FlagsConfig{
				flags.CommandGlobal: {
					"debug":            true,
					"disableAnalytics": false,
					"gitProtocol":      "https",
				},
				flags.CommandApply: {
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
			flags: flags.FlagsConfig{
				flags.CommandGlobal: {
					"unknownFlag": "value",
				},
				flags.CommandApply: {
					"anotherUnknownFlag": true,
				},
			},
			expectedErrors: 2, // Two unsupported flags
		},
		{
			name: "invalid git protocol",
			flags: flags.FlagsConfig{
				flags.CommandGlobal: {
					"gitProtocol": "invalid",
				},
			},
			expectedErrors: 1,
		},
		{
			name: "invalid force options",
			flags: flags.FlagsConfig{
				flags.CommandApply: {
					"force": []any{"invalid-option"},
				},
			},
			expectedErrors: 1,
		},
		{
			name: "invalid timeout",
			flags: flags.FlagsConfig{
				flags.CommandApply: {
					"timeout": -1,
				},
			},
			expectedErrors: 1,
		},
		{
			name: "conflicting vpn flags",
			flags: flags.FlagsConfig{
				flags.CommandApply: {
					"skipVpnConfirmation": true,
					"vpnAutoConnect":      true,
				},
			},
			expectedErrors: 1, // Conflicting flags
		},
		{
			name: "conflicting upgrade flags",
			flags: flags.FlagsConfig{
				flags.CommandApply: {
					"upgrade":     true,
					"upgradeNode": "worker1",
				},
			},
			expectedErrors: 1, // Conflicting flags
		},
		{
			name: "conflicting phase and startFrom",
			flags: flags.FlagsConfig{
				flags.CommandApply: {
					"phase":     "distribution",
					"startFrom": "infrastructure",
				},
			},
			expectedErrors: 1, // Conflicting flags
		},
		{
			name: "conflicting phase and postApplyPhases",
			flags: flags.FlagsConfig{
				flags.CommandApply: {
					"phase":           "distribution",
					"postApplyPhases": []any{"distribution"},
				},
			},
			expectedErrors: 1, // Conflicting flags
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validator.Validate(tt.flags)

			assert.Len(t, errors, tt.expectedErrors, "Expected %d errors, got %d: %v", tt.expectedErrors, len(errors), errors)
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
			assert.Equal(t, tt.expected, tt.err.Error())
		})
	}
}

func TestValidator_ErrorSeverityClassification(t *testing.T) {
	validator := flags.NewValidator()

	tests := []struct {
		name             string
		flags            flags.FlagsConfig
		expectedFatal    int
		expectedWarnings int
	}{
		{
			name: "fatal errors - invalid protocol and negative timeout",
			flags: flags.FlagsConfig{
				flags.CommandGlobal: {
					"gitProtocol": "ftp", // Fatal: invalid protocol
				},
				flags.CommandApply: {
					"timeout": -5, // Fatal: negative timeout
				},
			},
			expectedFatal:    2,
			expectedWarnings: 0,
		},
		{
			name: "fatal errors - unsupported flags",
			flags: flags.FlagsConfig{
				flags.CommandGlobal: {
					"unknownGlobalFlag": "value", // Fatal: unsupported
				},
				flags.CommandApply: {
					"unknownApplyFlag": true, // Fatal: unsupported
				},
			},
			expectedFatal:    2,
			expectedWarnings: 0,
		},
		{
			name: "fatal errors - conflicting flags",
			flags: flags.FlagsConfig{
				flags.CommandApply: {
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
			flags: flags.FlagsConfig{
				flags.CommandApply: {
					"force": []any{"invalid-option"}, // Fatal: invalid force option
				},
			},
			expectedFatal:    1,
			expectedWarnings: 0,
		},
		{
			name: "mixed fatal errors",
			flags: flags.FlagsConfig{
				flags.CommandGlobal: {
					"gitProtocol": "invalid", // Fatal: invalid protocol
					"unknownFlag": "value",   // Fatal: unsupported
				},
				flags.CommandApply: {
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

			assert.Equal(t, tt.expectedFatal, fatalCount, "Expected %d fatal errors, got %d", tt.expectedFatal, fatalCount)
			assert.Equal(t, tt.expectedWarnings, warningCount, "Expected %d warning errors, got %d", tt.expectedWarnings, warningCount)
		})
	}
}
