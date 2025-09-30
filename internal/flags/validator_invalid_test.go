// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package flags_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/flags"
)

func TestValidator_InvalidFlags_ShouldBeFatal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		flagsConfig    *flags.FlagsConfig
		expectedErrors int
		expectFatal    bool
		errorContains  []string
	}{
		{
			name: "invalid global flags should be fatal",
			flagsConfig: &flags.FlagsConfig{
				Global: map[string]any{
					"debug":          true,  // Valid.
					"invalidFlag":    "bad", // Invalid.
					"anotherBadFlag": 42,    // Invalid.
				},
			},
			expectedErrors: 2,
			expectFatal:    true,
			errorContains:  []string{"invalidFlag", "anotherBadFlag", "not supported"},
		},
		{
			name: "invalid apply flags should be fatal",
			flagsConfig: &flags.FlagsConfig{
				Apply: map[string]any{
					"dryRun":          true,  // Valid.
					"unsupportedFlag": "bad", // Invalid.
					"fakeApplyFlag":   false, // Invalid.
				},
			},
			expectedErrors: 2,
			expectFatal:    true,
			errorContains:  []string{"unsupportedFlag", "fakeApplyFlag", "not supported"},
		},
		{
			name: "all valid flags should pass",
			flagsConfig: &flags.FlagsConfig{
				Global: map[string]any{
					"debug":       true,
					"log":         "/tmp/furyctl.log",
					"gitProtocol": "ssh",
				},
			},
			expectedErrors: 0,
			expectFatal:    false,
		},
		{
			name: "mixed valid and invalid flags should be fatal",
			flagsConfig: &flags.FlagsConfig{
				Apply: map[string]any{
					"dryRun":  true,    // Valid.
					"phase":   "infra", // Valid.
					"badFlag": "oops",  // Invalid.
				},
			},
			expectedErrors: 1,
			expectFatal:    true,
			errorContains:  []string{"badFlag", "not supported"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			validator := flags.NewValidator()
			validationErrors := validator.Validate(tt.flagsConfig)

			assert.Len(t, validationErrors, tt.expectedErrors, "Expected %d validation errors", tt.expectedErrors)

			if tt.expectFatal {
				// Check that we have fatal errors.
				var fatalErrors []flags.ValidationError

				for _, err := range validationErrors {
					if err.Severity == flags.ValidationSeverityFatal {
						fatalErrors = append(fatalErrors, err)
					}
				}

				require.NotEmpty(t, fatalErrors, "Expected fatal validation errors")

				// Check error messages contain expected content.
				allErrorsText := ""
				for _, err := range fatalErrors {
					allErrorsText += err.Error() + " "
				}

				for _, expectedContent := range tt.errorContains {
					assert.Contains(t, allErrorsText, expectedContent, "Expected to find '%s' in error messages", expectedContent)
				}
			} else {
				// No fatal errors expected.
				for _, err := range validationErrors {
					assert.NotEqual(t, flags.ValidationSeverityFatal, err.Severity, "Did not expect fatal error: %v", err)
				}
			}
		})
	}
}

func TestValidator_InvalidFlags_ErrorMessages(t *testing.T) {
	t.Parallel()

	validator := flags.NewValidator()
	flagsConfig := &flags.FlagsConfig{
		Global: map[string]any{
			"invalidFlag": "value",
		},
	}

	validationErrors := validator.Validate(flagsConfig)

	require.Len(t, validationErrors, 1, "Expected exactly one validation error")

	err := validationErrors[0]
	assert.Equal(t, flags.ValidationSeverityFatal, err.Severity, "Expected fatal severity")
	assert.Equal(t, "global", err.Command)
	assert.Equal(t, "invalidFlag", err.Flag)
	assert.Equal(t, "value", err.Value)
	assert.Contains(t, err.Reason, "flag 'invalidFlag' is not supported for 'global' configuration")
	assert.Contains(t, err.Reason, "Check documentation for supported flags")
}

func TestValidator_SupportedFlags_DoNotCauseFatalErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		flagsConfig *flags.FlagsConfig
	}{
		{
			name: "all global flags should be valid",
			flagsConfig: &flags.FlagsConfig{
				Global: map[string]any{
					"debug":            true,
					"disableAnalytics": false,
					"noTty":            false,
					"workdir":          "/tmp",
					"outdir":           "/tmp/out",
					"log":              "/tmp/furyctl.log",
					"gitProtocol":      "https",
				},
			},
		},
		{
			name: "common apply flags should be valid",
			flagsConfig: &flags.FlagsConfig{
				Apply: map[string]any{
					"phase":              "infrastructure",
					"dryRun":             true,
					"skipDepsDownload":   false,
					"skipDepsValidation": false,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			validator := flags.NewValidator()
			validationErrors := validator.Validate(tt.flagsConfig)

			// Check that no fatal errors occurred due to unsupported flags.
			for _, err := range validationErrors {
				if err.Severity == flags.ValidationSeverityFatal {
					assert.NotContains(t, err.Reason, "not supported", "Unexpected fatal error for supported flag: %v", err)
				}
			}
		})
	}
}
