// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flags

import (
	"fmt"
	"strings"
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
	var errors []ValidationError

	if flags == nil {
		return errors
	}

	// Validate global flags
	if flags.Global != nil {
		errors = append(errors, v.validateCommandFlags(flags.Global, "global")...)
	}

	// Validate command-specific flags
	if flags.Apply != nil {
		errors = append(errors, v.validateCommandFlags(flags.Apply, "apply")...)
	}

	if flags.Delete != nil {
		errors = append(errors, v.validateCommandFlags(flags.Delete, "delete")...)
	}

	if flags.Create != nil {
		errors = append(errors, v.validateCommandFlags(flags.Create, "create")...)
	}

	if flags.Get != nil {
		errors = append(errors, v.validateCommandFlags(flags.Get, "get")...)
	}

	if flags.Diff != nil {
		errors = append(errors, v.validateCommandFlags(flags.Diff, "diff")...)
	}

	if flags.Tools != nil {
		errors = append(errors, v.validateCommandFlags(flags.Tools, "tools")...)
	}

	// Cross-validation: check for conflicting flags
	errors = append(errors, v.validateFlagCombinations(flags)...)

	return errors
}

// validateCommandFlags validates flags for a specific command.
func (v *Validator) validateCommandFlags(flagsMap map[string]any, command string) []ValidationError {
	var errors []ValidationError

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
		errors = append(errors, ValidationError{
			Command: command,
			Flag:    "",
			Value:   nil,
			Reason:  "unsupported command",
		})
		return errors
	}

	for flagName, value := range flagsMap {
		// Check if flag is supported
		flagInfo, supported := supportedFlagsMap[flagName]
		if !supported {
			errors = append(errors, ValidationError{
				Command: command,
				Flag:    flagName,
				Value:   value,
				Reason:  "unsupported flag for this command",
			})
			continue
		}

		// Validate the value type and content
		if err := v.validateFlagValue(flagName, value, flagInfo); err != nil {
			errors = append(errors, ValidationError{
				Command: command,
				Flag:    flagName,
				Value:   value,
				Reason:  err.Error(),
			})
		}
	}

	return errors
}

// validateFlagValue validates a single flag's value.
func (v *Validator) validateFlagValue(flagName string, value any, flagInfo FlagInfo) error {
	// Basic type validation
	switch flagInfo.Type {
	case FlagTypeBool:
		if _, ok := value.(bool); !ok {
			if str, ok := value.(string); ok {
				if str != "true" && str != "false" {
					return fmt.Errorf("boolean flag must be true or false, got: %v", value)
				}
			} else {
				return fmt.Errorf("expected boolean value, got: %T", value)
			}
		}

	case FlagTypeInt:
		switch value.(type) {
		case int, int64, float64:
			// Valid numeric types
		case string:
			// String representation of number, will be validated during conversion
		default:
			return fmt.Errorf("expected numeric value, got: %T", value)
		}

	case FlagTypeString:
		// Most types can be converted to string, so this is generally permissive

	case FlagTypeStringSlice:
		switch value.(type) {
		case []any, []string, string:
			// Valid slice types or comma-separated string
		default:
			return fmt.Errorf("expected array or string value, got: %T", value)
		}
	}

	// Specific flag validations
	return v.validateSpecificFlag(flagName, value)
}

// validateSpecificFlag performs validation specific to certain flags.
func (v *Validator) validateSpecificFlag(flagName string, value any) error {
	switch flagName {
	case "git-protocol":
		if str, ok := value.(string); ok {
			validProtocols := []string{"https", "ssh"}
			for _, valid := range validProtocols {
				if str == valid {
					return nil
				}
			}
			return fmt.Errorf("git-protocol must be one of: %s", strings.Join(validProtocols, ", "))
		}

	case "phase":
		if str, ok := value.(string); ok && str != "" {
			// TODO: Add phase validation once we have access to cluster phase constants
			// For now, accept any non-empty string
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
						return fmt.Errorf("invalid force option: %s, must be one of: %s", str, strings.Join(validForceOptions, ", "))
					}
				}
			}
		}

	case "timeout", "pod-running-check-timeout":
		if val, ok := value.(int); ok {
			if val <= 0 {
				return fmt.Errorf("%s must be a positive integer", flagName)
			}
		}
	}

	return nil
}

// validateFlagCombinations validates combinations of flags that might be incompatible.
func (*Validator) validateFlagCombinations(flags *FlagsConfig) []ValidationError {
	var errors []ValidationError

	// Check apply-specific flag combinations
	if flags.Apply != nil {
		// Check skip-vpn-confirmation vs vpn-auto-connect
		if skipVpn, hasSkipVpn := flags.Apply["skip-vpn-confirmation"]; hasSkipVpn {
			if autoConnect, hasAutoConnect := flags.Apply["vpn-auto-connect"]; hasAutoConnect {
				if skipVpnBool, ok := skipVpn.(bool); ok && skipVpnBool {
					if autoConnectBool, ok := autoConnect.(bool); ok && autoConnectBool {
						errors = append(errors, ValidationError{
							Command: "apply",
							Flag:    "vpn-auto-connect",
							Value:   autoConnect,
							Reason:  "cannot be used together with skip-vpn-confirmation",
						})
					}
				}
			}
		}

		// Check upgrade vs upgrade-node
		if upgrade, hasUpgrade := flags.Apply["upgrade"]; hasUpgrade {
			if upgradeNode, hasUpgradeNode := flags.Apply["upgrade-node"]; hasUpgradeNode {
				if upgradeBool, ok := upgrade.(bool); ok && upgradeBool {
					if upgradeNodeStr, ok := upgradeNode.(string); ok && upgradeNodeStr != "" {
						errors = append(errors, ValidationError{
							Command: "apply",
							Flag:    "upgrade-node",
							Value:   upgradeNode,
							Reason:  "cannot be used together with upgrade",
						})
					}
				}
			}
		}

		// Check phase vs start-from
		if phase, hasPhase := flags.Apply["phase"]; hasPhase {
			if startFrom, hasStartFrom := flags.Apply["start-from"]; hasStartFrom {
				if phaseStr, ok := phase.(string); ok && phaseStr != "" && phaseStr != "all" {
					if startFromStr, ok := startFrom.(string); ok && startFromStr != "" {
						errors = append(errors, ValidationError{
							Command: "apply",
							Flag:    "start-from",
							Value:   startFrom,
							Reason:  "cannot be used together with phase flag",
						})
					}
				}
			}
		}

		// Check phase vs post-apply-phases
		if phase, hasPhase := flags.Apply["phase"]; hasPhase {
			if postApplyPhases, hasPostApply := flags.Apply["post-apply-phases"]; hasPostApply {
				if phaseStr, ok := phase.(string); ok && phaseStr != "" && phaseStr != "all" {
					if phases, ok := postApplyPhases.([]any); ok && len(phases) > 0 {
						errors = append(errors, ValidationError{
							Command: "apply",
							Flag:    "post-apply-phases",
							Value:   postApplyPhases,
							Reason:  "cannot be used together with phase flag",
						})
					}
				}
			}
		}
	}

	return errors
}

// ValidateFlagValue is a public wrapper for testing the flag value validation.
func (v *Validator) ValidateFlagValue(flagName string, value any, flagInfo FlagInfo) error {
	return v.validateFlagValue(flagName, value, flagInfo)
}
