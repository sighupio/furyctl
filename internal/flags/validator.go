// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flags

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/sirupsen/logrus"
)

// Static error definitions for linting compliance.
var (
	ErrInvalidProtocol       = errors.New("invalid git protocol")
	ErrInvalidForceOption    = errors.New("invalid force option")
	ErrMustBePositiveInteger = errors.New("must be a positive integer")
	ErrConflictingFlags      = errors.New("conflicting flags detected")
	ErrInvalidBooleanValue   = errors.New("invalid boolean value")
	ErrExpectedBooleanType   = errors.New("expected boolean type")
	ErrExpectedNumericType   = errors.New("expected numeric type")
	ErrExpectedArrayOrString = errors.New("expected array or string type")
)

// Validator handles validation of flags configuration.
type Validator struct {
	supportedFlags SupportedFlags
}

// NewValidator creates a new flags validator.
func NewValidator() *Validator {
	return &Validator{
		supportedFlags: GetSupportedFlags(),
	}
}

// Validate validates the entire flags configuration.
func (v *Validator) Validate(flags FlagsConfig) []ValidationError {
	validationErrors := make([]ValidationError, 0, len(flags))

	// Sort the sections so that the messages keep the same order between runs.
	for _, section := range slices.Sorted(maps.Keys(flags)) {
		validationErrors = append(validationErrors, v.validateCommandFlags(flags[section], section)...)
	}

	// Cross-validation: check for conflicting flags.
	validationErrors = append(validationErrors, v.validateFlagCombinations(flags)...)

	return validationErrors
}

// validateCommandFlags validates flags for a specific command.
func (v *Validator) validateCommandFlags(flagsMap map[string]any, command string) []ValidationError {
	var validationErrors []ValidationError

	supportedFlagsMap, supported := v.supportedFlags[command]
	if !supported {
		return []ValidationError{{
			Command:  command,
			Flag:     "",
			Value:    nil,
			Reason:   "unsupported command",
			Severity: ValidationSeverityFatal,
		}}
	}

	for flagName, value := range flagsMap {
		// Check if flag is supported.
		flagType, supported := supportedFlagsMap[flagName]
		if !supported {
			validationErrors = append(validationErrors, ValidationError{
				Command: command,
				Flag:    flagName,
				Value:   value,
				Reason: fmt.Sprintf("flag '%s' is not supported for '%s' %s. "+
					"Check documentation for supported flags.", flagName, command, func() string {
					if command == CommandGlobal {
						return "configuration"
					}

					return "command"
				}()),
				Severity: ValidationSeverityFatal,
			})

			continue
		}

		// Validate the value type and content.
		if err := v.validateFlagValue(flagName, value, flagType); err != nil {
			validationErrors = append(validationErrors, ValidationError{
				Command:  command,
				Flag:     flagName,
				Value:    value,
				Reason:   err.Error(),
				Severity: getValidationSeverity(flagName, err),
			})
		}
	}

	return validationErrors
}

// validateFlagValue validates a single flag's value.
func (v *Validator) validateFlagValue(flagName string, value any, flagType FlagType) error {
	// Basic type validation.
	switch flagType {
	case FlagTypeBool:
		if _, ok := value.(bool); !ok {
			if str, ok := value.(string); !ok {
				return fmt.Errorf("%w: got %T", ErrExpectedBooleanType, value)
			} else if str != "true" && str != "false" {
				return fmt.Errorf("%w: got %v", ErrInvalidBooleanValue, value)
			}
		}

	case FlagTypeInt:
		switch value.(type) {
		case int, int64, float64:
			// Valid numeric types.
		case string:
			// String representation of number, will be validated during conversion.
		default:
			return fmt.Errorf("%w: got %T", ErrExpectedNumericType, value)
		}

	case FlagTypeStringSlice:
		switch value.(type) {
		case []any, []string, string:
			// Types are valid - no action needed.
		default:
			return fmt.Errorf("%w: got %T", ErrExpectedArrayOrString, value)
		}

	case FlagTypeString, FlagTypeDuration:
		// No validation needed - most types can be converted to string/duration.
		// This is intentionally permissive for these types.
		_ = value // No-op to satisfy WSL linter.

	default:
		logrus.Debugf("flag %q has unknown type %v; skipping basic type validation", flagName, flagType)
	}

	// Specific flag validations.
	return v.validateSpecificFlag(flagName, value)
}

// validateSpecificFlag performs validation specific to certain flags.
func (*Validator) validateSpecificFlag(flagName string, value any) error {
	switch flagName {
	case "gitProtocol":
		if str, ok := value.(string); ok {
			validProtocols := []string{"https", "ssh"}
			if slices.Contains(validProtocols, str) {
				return nil
			}

			return fmt.Errorf("%w: got '%s', must be one of: %s", ErrInvalidProtocol, str, strings.Join(validProtocols, ", "))
		}
		return nil

	case "phase":
		if str, ok := value.(string); ok && str != "" {
			//nolint:godox // TODO acceptable here - phase validation depends on external constants
			// TODO: Add phase validation once we have access to cluster phase constants.
			// For now, accept any non-empty string.
			_ = str // Prevent unused variable warning.
		}
		return nil

	case "force":
		if slice, ok := value.([]any); ok {
			validForceOptions := []string{"all", "upgrades", "migrations", "pods-running-check"}

			for _, item := range slice {
				str, ok := item.(string)
				if ok && !slices.Contains(validForceOptions, str) {
					return fmt.Errorf("%w: got '%s', must be one of: %s",
						ErrInvalidForceOption, str, strings.Join(validForceOptions, ", "))
				}
			}
		}
		return nil

	case "timeout", "podRunningCheckTimeout":
		if val, ok := value.(int); ok {
			if val <= 0 {
				return fmt.Errorf("%w: %s must be greater than 0, got %v", ErrMustBePositiveInteger, flagName, val)
			}
		}
		return nil

	default:
		logrus.Debugf("flag %q has no specific validation rule", flagName)
		return nil
	}
}

// validateFlagCombinations validates combinations of flags that might be incompatible.
func (*Validator) validateFlagCombinations(flags FlagsConfig) []ValidationError {
	apply := flags[CommandApply]

	isTrue := func(name string) bool { v, ok := apply[name].(bool); return ok && v }
	isSet := func(name string) bool { v, ok := apply[name].(string); return ok && v != "" }
	hasItems := func(name string) bool { v, ok := apply[name].([]any); return ok && len(v) > 0 }
	phaseSet := isSet("phase") && apply["phase"] != "all"

	var errs []ValidationError

	add := func(flag, reason string) {
		errs = append(errs, ValidationError{
			Command: CommandApply, Flag: flag, Value: apply[flag], Reason: reason, Severity: ValidationSeverityFatal,
		})
	}

	if isTrue("skipVpnConfirmation") && isTrue("vpnAutoConnect") {
		add("vpnAutoConnect", "vpnAutoConnect=true conflicts with skipVpnConfirmation=true. Use only one of these flags.")
	}

	if isTrue("upgrade") && isSet("upgradeNode") {
		add("upgradeNode", "upgradeNode cannot be used when upgrade=true. "+
			"Use either 'upgrade' for all nodes or 'upgradeNode' for a specific node.")
	}

	if phaseSet && isSet("startFrom") {
		add("startFrom", "startFrom cannot be used when phase is specified (and not 'all'). "+
			"Use either 'phase' or 'startFrom', not both.")
	}

	if phaseSet && hasItems("postApplyPhases") {
		add("postApplyPhases", "postApplyPhases cannot be used when phase is specified (and not 'all'). "+
			"Use either 'phase' or 'postApplyPhases', not both.")
	}

	return errs
}

// getValidationSeverity determines the severity level for a validation error.
func getValidationSeverity(flagName string, err error) ValidationSeverity {
	// Critical errors that should stop execution.
	if errors.Is(err, ErrInvalidProtocol) ||
		errors.Is(err, ErrInvalidForceOption) ||
		errors.Is(err, ErrMustBePositiveInteger) ||
		errors.Is(err, ErrConflictingFlags) {
		return ValidationSeverityFatal
	}

	// Timeout validation errors are always fatal.
	if flagName == "timeout" || flagName == "podRunningCheckTimeout" {
		return ValidationSeverityFatal
	}

	// Type validation errors for critical types are fatal.
	if errors.Is(err, ErrExpectedBooleanType) ||
		errors.Is(err, ErrExpectedNumericType) ||
		errors.Is(err, ErrInvalidBooleanValue) {
		return ValidationSeverityFatal
	}

	// Default to warning for less critical validation issues.
	return ValidationSeverityWarning
}
