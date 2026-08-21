// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flags

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/samber/lo"
	"github.com/spf13/viper"
)

// Command name constants. Each one is a section of the `flags` field of furyctl.yaml.
const (
	CommandGlobal   = "global"
	CommandApply    = "apply"
	CommandDelete   = "delete"
	CommandCreate   = "create"
	CommandGet      = "get"
	CommandDiff     = "diff"
	CommandValidate = "validate"
	CommandDownload = "download"
	CommandConnect  = "connect"
	CommandRenew    = "renew"
	CommandDump     = "dump"
)

// Static error definitions for linting compliance.
var (
	ErrTypeConversion      = errors.New("type conversion failed")
	ErrUnsupportedFlagType = errors.New("unsupported flag type")
	ErrBoolConversion      = errors.New("cannot convert to bool")
	ErrIntConversion       = errors.New("cannot convert to int")
	ErrUnsupportedCommand  = errors.New("unsupported command")
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

// CamelToKebab converts a camelCase string to kebab-case.
// For example: "distroLocation" -> "distro-location".
func CamelToKebab(s string) string {
	var result strings.Builder

	for i, r := range s {
		if i > 0 && unicode.IsUpper(r) {
			_, _ = result.WriteRune('-')
		}

		_, _ = result.WriteRune(unicode.ToLower(r))
	}

	return result.String()
}

// MergeIntoViper merges flags from the configuration into viper with the lowest priority.
// This ensures the priority order: furyctl.yaml < environment variables < command line flags.
func (m *Merger) MergeIntoViper(flags *FlagsConfig, command string) error {
	if flags == nil {
		return nil
	}

	// Merge global flags first.
	if err := m.mergeCommandFlags(flags.Global, CommandGlobal); err != nil {
		return fmt.Errorf("error merging global flags: %w", err)
	}

	// Merge command-specific flags.
	var commandFlags map[string]any

	switch command {
	case CommandApply:
		commandFlags = flags.Apply

	case CommandDelete:
		commandFlags = flags.Delete

	case CommandCreate:
		commandFlags = flags.Create

	case CommandGet:
		commandFlags = flags.Get

	case CommandDiff:
		commandFlags = flags.Diff

	case CommandValidate:
		commandFlags = flags.Validate

	case CommandDownload:
		commandFlags = flags.Download

	case CommandConnect:
		commandFlags = flags.Connect

	case CommandRenew:
		commandFlags = flags.Renew

	case CommandDump:
		commandFlags = flags.Dump

	default:
		// Unknown command, skip command-specific flags.
		return nil
	}

	if commandFlags != nil {
		if err := m.mergeCommandFlags(commandFlags, command); err != nil {
			return fmt.Errorf("error merging %s flags: %w", command, err)
		}
	}

	return nil
}

// ConvertValue converts a value to the expected type for the flag.
func (*Merger) ConvertValue(value any, expectedType FlagType) (any, error) {
	switch expectedType {
	case FlagTypeString, FlagTypeDuration:
		// Duration is treated as string here; viper handles the actual conversion later.
		return fmt.Sprintf("%v", value), nil

	case FlagTypeBool:
		switch v := value.(type) {
		case bool:
			return v, nil

		case string:
			result, err := strconv.ParseBool(v)
			if err != nil {
				return false, fmt.Errorf("%w: %w", ErrBoolConversion, err)
			}

			return result, nil

		default:
			return false, fmt.Errorf("%w: got %T", ErrBoolConversion, value)
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
			result, err := strconv.Atoi(v)
			if err != nil {
				return 0, fmt.Errorf("%w: %w", ErrIntConversion, err)
			}

			return result, nil

		default:
			return 0, fmt.Errorf("%w: got %T", ErrIntConversion, value)
		}

	case FlagTypeStringSlice:
		switch v := value.(type) {
		case []any:
			return lo.Map(v, func(item any, _ int) string {
				return fmt.Sprintf("%v", item)
			}), nil

		case []string:
			return v, nil

		case string:
			// Handle comma-separated string.
			if v == "" {
				return []string{}, nil
			}

			return strings.Split(v, ","), nil

		default:
			return []string{}, ErrTypeConversion
		}

	default:
		return nil, ErrUnsupportedFlagType
	}
}

// MergeGlobalFlags is a convenience method to merge only global flags.
func (m *Merger) MergeGlobalFlags(flags *FlagsConfig) error {
	if flags == nil || flags.Global == nil {
		return nil
	}

	return m.mergeCommandFlags(flags.Global, CommandGlobal)
}

// GetSupportedFlagsForCommand returns the supported flags for a specific command.
func (m *Merger) GetSupportedFlagsForCommand(command string) map[string]FlagInfo {
	switch command {
	case CommandGlobal:
		return m.supportedFlags.Global

	case CommandApply:
		return m.supportedFlags.Apply

	case CommandDelete:
		return m.supportedFlags.Delete

	case CommandCreate:
		return m.supportedFlags.Create

	case CommandGet:
		return m.supportedFlags.Get

	case CommandDiff:
		return m.supportedFlags.Diff

	case CommandValidate:
		return m.supportedFlags.Validate

	case CommandDownload:
		return m.supportedFlags.Download

	case CommandConnect:
		return m.supportedFlags.Connect

	case CommandRenew:
		return m.supportedFlags.Renew

	case CommandDump:
		return m.supportedFlags.Dump

	default:
		return nil
	}
}

// mergeCommandFlags merges flags for a specific command into viper.
func (m *Merger) mergeCommandFlags(flagsMap map[string]any, command string) error {
	var supportedFlagsMap map[string]FlagInfo

	switch command {
	case CommandGlobal:
		supportedFlagsMap = m.supportedFlags.Global

	case CommandApply:
		supportedFlagsMap = m.supportedFlags.Apply

	case CommandDelete:
		supportedFlagsMap = m.supportedFlags.Delete

	case CommandCreate:
		supportedFlagsMap = m.supportedFlags.Create

	case CommandGet:
		supportedFlagsMap = m.supportedFlags.Get

	case CommandDiff:
		supportedFlagsMap = m.supportedFlags.Diff

	case CommandValidate:
		supportedFlagsMap = m.supportedFlags.Validate

	case CommandDownload:
		supportedFlagsMap = m.supportedFlags.Download

	case CommandConnect:
		supportedFlagsMap = m.supportedFlags.Connect

	case CommandRenew:
		supportedFlagsMap = m.supportedFlags.Renew

	case CommandDump:
		supportedFlagsMap = m.supportedFlags.Dump

	default:
		return fmt.Errorf("%w: %s", ErrUnsupportedCommand, command)
	}

	for flagName, value := range flagsMap {
		// Check if the flag is supported.
		flagInfo, supported := supportedFlagsMap[flagName]
		if !supported {
			// Log warning but don't fail - might be a new flag.
			continue
		}

		// Convert and validate the value.
		convertedValue, err := m.ConvertValue(value, flagInfo.Type)
		if err != nil {
			return fmt.Errorf("error converting flag %s: %w", flagName, err)
		}

		// Convert camelCase flag name to kebab-case for viper.
		viperKey := CamelToKebab(flagName)

		// Set the value in viper only if it's not already set.
		// This preserves the priority: env vars and command line flags take precedence.
		if !viper.IsSet(viperKey) {
			viper.Set(viperKey, convertedValue)
		}
	}

	return nil
}
