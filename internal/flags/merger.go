// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flags

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

// Merger handles merging flags from configuration file into viper with proper priority.
type Merger struct {
	supportedFlags SupportedFlags
}

// NewMerger creates a new flags merger.
func NewMerger() *Merger {
	return &Merger{
		supportedFlags: GetSupportedFlags(),
	}
}

// MergeIntoViper merges flags from the configuration into viper with the lowest priority.
// This ensures the priority order: furyctl.yaml < environment variables < command line flags.
func (m *Merger) MergeIntoViper(flags *FlagsConfig, command string) error {
	if flags == nil {
		return nil
	}

	// Merge global flags first
	if err := m.mergeCommandFlags(flags.Global, "global"); err != nil {
		return fmt.Errorf("error merging global flags: %w", err)
	}

	// Merge command-specific flags
	var commandFlags map[string]interface{}
	switch command {
	case "apply":
		commandFlags = flags.Apply
	case "delete":
		commandFlags = flags.Delete
	case "create":
		commandFlags = flags.Create
	case "get":
		commandFlags = flags.Get
	case "diff":
		commandFlags = flags.Diff
	case "tools":
		commandFlags = flags.Tools
	default:
		// Unknown command, skip command-specific flags
		return nil
	}

	if commandFlags != nil {
		if err := m.mergeCommandFlags(commandFlags, command); err != nil {
			return fmt.Errorf("error merging %s flags: %w", command, err)
		}
	}

	return nil
}

// mergeCommandFlags merges flags for a specific command into viper.
func (m *Merger) mergeCommandFlags(flagsMap map[string]any, command string) error {
	var supportedFlagsMap map[string]FlagInfo

	switch command {
	case "global":
		supportedFlagsMap = m.supportedFlags.Global
	case "apply":
		supportedFlagsMap = m.supportedFlags.Apply
	case "delete":
		supportedFlagsMap = m.supportedFlags.Delete
	case "create":
		supportedFlagsMap = m.supportedFlags.Create
	case "get":
		supportedFlagsMap = m.supportedFlags.Get
	case "diff":
		supportedFlagsMap = m.supportedFlags.Diff
	case "tools":
		supportedFlagsMap = m.supportedFlags.Tools
	default:
		return fmt.Errorf("unsupported command: %s", command)
	}

	for flagName, value := range flagsMap {
		// Check if the flag is supported
		flagInfo, supported := supportedFlagsMap[flagName]
		if !supported {
			// Log warning but don't fail - might be a new flag
			continue
		}

		// Convert and validate the value
		convertedValue, err := m.convertValue(value, flagInfo.Type)
		if err != nil {
			return fmt.Errorf("error converting flag %s: %w", flagName, err)
		}

		// Set the value in viper only if it's not already set
		// This preserves the priority: env vars and command line flags take precedence
		if !viper.IsSet(flagName) {
			viper.Set(flagName, convertedValue)
		}
	}

	return nil
}

// convertValue converts a value to the expected type for the flag.
func (m *Merger) convertValue(value any, expectedType FlagType) (any, error) {
	switch expectedType {
	case FlagTypeString:
		return fmt.Sprintf("%v", value), nil

	case FlagTypeBool:
		switch v := value.(type) {
		case bool:
			return v, nil
		case string:
			return strconv.ParseBool(v)
		default:
			return false, fmt.Errorf("cannot convert %T to bool", value)
		}

	case FlagTypeInt:
		switch v := value.(type) {
		case int:
			return v, nil
		case int64:
			return int(v), nil
		case float64:
			return int(v), nil
		case string:
			return strconv.Atoi(v)
		default:
			return 0, fmt.Errorf("cannot convert %T to int", value)
		}

	case FlagTypeStringSlice:
		switch v := value.(type) {
		case []any:
			result := make([]string, len(v))
			for i, item := range v {
				result[i] = fmt.Sprintf("%v", item)
			}
			return result, nil
		case []string:
			return v, nil
		case string:
			// Handle comma-separated string
			if v == "" {
				return []string{}, nil
			}
			return strings.Split(v, ","), nil
		default:
			return []string{}, fmt.Errorf("cannot convert %T to []string", value)
		}

	case FlagTypeDuration:
		// For now, treat duration as string and let viper handle the conversion
		return fmt.Sprintf("%v", value), nil

	default:
		return nil, fmt.Errorf("unsupported flag type: %s", expectedType)
	}
}

// MergeGlobalFlags is a convenience method to merge only global flags.
func (m *Merger) MergeGlobalFlags(flags *FlagsConfig) error {
	if flags == nil || flags.Global == nil {
		return nil
	}

	return m.mergeCommandFlags(flags.Global, "global")
}

// GetSupportedFlagsForCommand returns the supported flags for a specific command.
func (m *Merger) GetSupportedFlagsForCommand(command string) map[string]FlagInfo {
	switch command {
	case "global":
		return m.supportedFlags.Global
	case "apply":
		return m.supportedFlags.Apply
	case "delete":
		return m.supportedFlags.Delete
	case "create":
		return m.supportedFlags.Create
	case "get":
		return m.supportedFlags.Get
	case "diff":
		return m.supportedFlags.Diff
	case "tools":
		return m.supportedFlags.Tools
	default:
		return nil
	}
}
