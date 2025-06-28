// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flags

import (
	"errors"
	"fmt"
	"strings"
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
func (v *Validator) Validate(flags *FlagsConfig) []ValidationError {
	var validationErrors []ValidationError

	if flags == nil {
		return validationErrors
	}

	// Validate global flags
	if flags.Global != nil {
		validationErrors = append(validationErrors, v.validateCommandFlags(flags.Global, "global")...)
	}

	// Validate command-specific flags
	if flags.Apply != nil {
		validationErrors = append(validationErrors, v.validateCommandFlags(flags.Apply, "apply")...)
	}

	if flags.Delete != nil {
		validationErrors = append(validationErrors, v.validateCommandFlags(flags.Delete, "delete")...)
	}

	if flags.Create != nil {
		validationErrors = append(validationErrors, v.validateCommandFlags(flags.Create, "create")...)
	}

	if flags.Get != nil {
		validationErrors = append(validationErrors, v.validateCommandFlags(flags.Get, "get")...)
	}

	if flags.Diff != nil {
		validationErrors = append(validationErrors, v.validateCommandFlags(flags.Diff, "diff")...)
	}

	if flags.Tools != nil {
		validationErrors = append(validationErrors, v.validateCommandFlags(flags.Tools, "tools")...)
	}

	// Cross-validation: check for conflicting flags
	validationErrors = append(validationErrors, v.validateFlagCombinations(flags)...)

	return validationErrors
}

// validateCommandFlags validates flags for a specific command.
func (v *Validator) validateCommandFlags(flagsMap map[string]any, command string) []ValidationError {
	var validationErrors []ValidationError

	var supportedFlagsMap map[string]FlagInfo

	switch command {
	case "global":
		supportedFlagsMap = v.supportedFlags.Global
	case "apply":
		supportedFlagsMap = v.supportedFlags.Apply
	case "delete":
		supportedFlagsMap = v.supportedFlags.Delete
	case "create":
		supportedFlagsMap = v.supportedFlags.Create
	case "get":
		supportedFlagsMap = v.supportedFlags.Get
	case "diff":
		supportedFlagsMap = v.supportedFlags.Diff
	case "tools":
		supportedFlagsMap = v.supportedFlags.Tools
	default:
		validationErrors = append(validationErrors, ValidationError{
			Command: command,
			Flag:    "",
			Value:   nil,
			Reason:  "unsupported command",
		})

		return validationErrors
	}

	for flagName, value := range flagsMap {
		// Check if flag is supported
		flagInfo, supported := supportedFlagsMap[flagName]
		if !supported {
			validationErrors = append(validationErrors, ValidationError{
				Command: command,
				Flag:    flagName,
				Value:   value,
				Reason:  "unsupported flag for this command",
			})

			continue
		}

		// Validate the value type and content
		if err := v.validateFlagValue(flagName, value, flagInfo); err != nil {
			validationErrors = append(validationErrors, ValidationError{
				Command: command,
				Flag:    flagName,
				Value:   value,
				Reason:  err.Error(),
			})
		}
	}

	return validationErrors
}

// validateFlagValue validates a single flag's value.
func (v *Validator) validateFlagValue(flagName string, value any, flagInfo FlagInfo) error {
	// Basic type validation
	switch flagInfo.Type {
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
			// Valid numeric types
		case string:
			// String representation of number, will be validated during conversion
		default:
			return fmt.Errorf("%w: got %T", ErrExpectedNumericType, value)
		}

	case FlagTypeString:
		// Most types can be converted to string, so this is generally permissive

	case FlagTypeStringSlice:
		switch value.(type) {
		case []any, []string, string:
			// Valid slice types or comma-separated string
		default:
			return fmt.Errorf("%w: got %T", ErrExpectedArrayOrString, value)
		}

	case FlagTypeDuration:
		// Duration validation - most types can be converted to string for duration parsing
		// Detailed validation will be done during conversion
	}

	// Specific flag validations
	return v.validateSpecificFlag(flagName, value)
}

// validateSpecificFlag performs validation specific to certain flags.
func (*Validator) validateSpecificFlag(flagName string, value any) error {
	switch flagName {
	case "gitProtocol":
		if str, ok := value.(string); ok {
			validProtocols := []string{"https", "ssh"}
			for _, valid := range validProtocols {
				if str == valid {
					return nil
				}
			}

			return fmt.Errorf("%w: must be one of: %s", ErrInvalidProtocol, strings.Join(validProtocols, ", "))
		}

	case "phase":
		if str, ok := value.(string); ok && str != "" {
			// TODO: Add phase validation once we have access to cluster phase constants
			// For now, accept any non-empty string
			_ = str // Prevent unused variable warning
		}

	case "force":
		if slice, ok := value.([]any); ok {
			validForceOptions := []string{"all", "upgrades", "migrations", "pods-running-check"}

			for _, item := range slice {
				if str, ok := item.(string); ok {
					found := false

					for _, valid := range validForceOptions {
						if str == valid {
							found = true

							break
						}
					}

					if !found {
						return fmt.Errorf("%w: %s, must be one of: %s", ErrInvalidForceOption, str, strings.Join(validForceOptions, ", "))
					}
				}
			}
		}

	case "timeout", "podRunningCheckTimeout":
		if val, ok := value.(int); ok {
			if val <= 0 {
				return fmt.Errorf("%w: %s", ErrMustBePositiveInteger, flagName)
			}
		}
	}

	return nil
}

// validateFlagCombinations validates combinations of flags that might be incompatible.
func (*Validator) validateFlagCombinations(flags *FlagsConfig) []ValidationError {
	var validationErrors []ValidationError

	// Check apply-specific flag combinations
	if flags.Apply != nil {
		// Check skipVpnConfirmation vs vpnAutoConnect
		if skipVpn, hasSkipVpn := flags.Apply["skipVpnConfirmation"]; hasSkipVpn {
			if autoConnect, hasAutoConnect := flags.Apply["vpnAutoConnect"]; hasAutoConnect {
				if skipVpnBool, ok := skipVpn.(bool); ok && skipVpnBool {
					if autoConnectBool, ok := autoConnect.(bool); ok && autoConnectBool {
						validationErrors = append(validationErrors, ValidationError{
							Command: "apply",
							Flag:    "vpnAutoConnect",
							Value:   autoConnect,
							Reason:  "cannot be used together with skipVpnConfirmation",
						})
					}
				}
			}
		}

		// Check upgrade vs upgradeNode
		if upgrade, hasUpgrade := flags.Apply["upgrade"]; hasUpgrade {
			if upgradeNode, hasUpgradeNode := flags.Apply["upgradeNode"]; hasUpgradeNode {
				if upgradeBool, ok := upgrade.(bool); ok && upgradeBool {
					if upgradeNodeStr, ok := upgradeNode.(string); ok && upgradeNodeStr != "" {
						validationErrors = append(validationErrors, ValidationError{
							Command: "apply",
							Flag:    "upgradeNode",
							Value:   upgradeNode,
							Reason:  "cannot be used together with upgrade",
						})
					}
				}
			}
		}

		// Check phase vs startFrom
		if phase, hasPhase := flags.Apply["phase"]; hasPhase {
			if startFrom, hasStartFrom := flags.Apply["startFrom"]; hasStartFrom {
				if phaseStr, ok := phase.(string); ok && phaseStr != "" && phaseStr != "all" {
					if startFromStr, ok := startFrom.(string); ok && startFromStr != "" {
						validationErrors = append(validationErrors, ValidationError{
							Command: "apply",
							Flag:    "startFrom",
							Value:   startFrom,
							Reason:  "cannot be used together with phase flag",
						})
					}
				}
			}
		}

		// Check phase vs postApplyPhases
		if phase, hasPhase := flags.Apply["phase"]; hasPhase {
			if postApplyPhases, hasPostApply := flags.Apply["postApplyPhases"]; hasPostApply {
				if phaseStr, ok := phase.(string); ok && phaseStr != "" && phaseStr != "all" {
					if phases, ok := postApplyPhases.([]any); ok && len(phases) > 0 {
						validationErrors = append(validationErrors, ValidationError{
							Command: "apply",
							Flag:    "postApplyPhases",
							Value:   postApplyPhases,
							Reason:  "cannot be used together with phase flag",
						})
					}
				}
			}
		}
	}

	return validationErrors
}

// ValidateFlagValue is a public wrapper for testing the flag value validation.
func (v *Validator) ValidateIndividualFlag(flagName string, value any, flagInfo FlagInfo) error {
	return v.validateFlagValue(flagName, value, flagInfo)
}
