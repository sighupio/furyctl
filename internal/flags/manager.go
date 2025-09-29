// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flags

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"github.com/sighupio/furyctl/internal/parser"
)

// Static error definitions for linting compliance.
var (
	ErrFlagsValidationFailed       = errors.New("flags validation failed")
	ErrGlobalFlagsValidationFailed = errors.New("global flags validation failed")
)

// Manager coordinates flags loading, validation, and merging operations.
type Manager struct {
	loader    *Loader
	merger    *Merger
	validator *Validator
}

// NewManager creates a new flags manager.
func NewManager(baseDir string) *Manager {
	return &Manager{
		loader:    NewLoader(baseDir),
		merger:    NewMerger(),
		validator: NewValidator(),
	}
}

// LoadAndMergeFlags loads flags from configuration file and merges them into viper.
// This is the main entry point for the flags system.
func (m *Manager) LoadAndMergeFlags(configPath, command string) error {
	// Resolve absolute path for the config.
	absConfigPath, err := filepath.Abs(configPath)
	if err != nil {
		return fmt.Errorf("failed to resolve config path: %w", err)
	}

	// Load flags from configuration file.
	result, err := m.loader.LoadFromFile(absConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load flags: %w", err)
	}

	// Check for errors in the result.
	if len(result.Errors) > 0 {
		for _, loadErr := range result.Errors {
			// Check if this is a critical error that should stop execution.
			if isCriticalError(loadErr) {
				return fmt.Errorf("failed to load flags: %w", loadErr)
			}
			// Non-critical errors (like config file not found) should not stop execution.
			logrus.Debugf("Flags loading error: %v", loadErr)
		}
	}

	// If no flags configuration found, nothing to merge.
	if result.Flags == nil {
		logrus.Debugf("No flags configuration found in %s", configPath)

		return nil
	}

	// Validate the flags configuration.
	validationErrors := m.validator.Validate(result.Flags)
	if err := m.handleValidationErrors(validationErrors, ErrFlagsValidationFailed, "flags configuration"); err != nil {
		return err
	}

	// Continue with merging despite validation errors (warnings only).

	// Merge flags into viper with lowest priority.
	if err := m.merger.MergeIntoViper(result.Flags, command); err != nil {
		return fmt.Errorf("failed to merge flags: %w", err)
	}

	logrus.Debugf("Successfully loaded and merged flags from %s for command %s", configPath, command)

	return nil
}

// LoadAndMergeGlobalFlags loads and merges only global flags.
func (m *Manager) LoadAndMergeGlobalFlags(configPath string) error {
	// Resolve absolute path for the config.
	absConfigPath, err := filepath.Abs(configPath)
	if err != nil {
		return fmt.Errorf("failed to resolve config path: %w", err)
	}

	// Load flags from configuration file.
	result, err := m.loader.LoadFromFile(absConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load global flags: %w", err)
	}

	// Check for errors in the result.
	if len(result.Errors) > 0 {
		for _, loadErr := range result.Errors {
			// For global flags, critical errors should still be fatal before log file creation.
			if isCriticalError(loadErr) {
				return fmt.Errorf("failed to load global flags: %w", loadErr)
			}
			// Non-critical errors should not stop execution.
			logrus.Debugf("Global flags loading error: %v", loadErr)
		}
	}

	// If no flags configuration found, nothing to merge.
	if result.Flags == nil {
		logrus.Debugf("No global flags configuration found in %s", configPath)

		return nil
	}

	// Validate only global flags.
	if result.Flags.Global != nil {
		validationErrors := m.validator.validateCommandFlags(result.Flags.Global, "global")
		if err := m.handleValidationErrors(
			validationErrors, ErrGlobalFlagsValidationFailed, "global flags configuration",
		); err != nil {
			return err
		}
	}

	// Merge only global flags.
	if err := m.merger.MergeGlobalFlags(result.Flags); err != nil {
		return fmt.Errorf("failed to merge global flags: %w", err)
	}

	logrus.Debugf("Successfully loaded and merged global flags from %s", configPath)

	return nil
}

// TryLoadFromCurrentDirectory attempts to load flags from the current working directory.
func (m *Manager) TryLoadFromCurrentDirectory(command string) error {
	result, err := m.loader.LoadFromDirectory(".")
	if err != nil {
		// This is expected if no config file exists.
		logrus.Debugf("No configuration file found in current directory: %v", err)

		return nil
	}

	// Check for errors in the result.
	if len(result.Errors) > 0 {
		for _, loadErr := range result.Errors {
			// This is expected if no config file exists.
			logrus.Debugf("No configuration file found in current directory: %v", loadErr)
		}
	}

	if result.Flags == nil {
		logrus.Debugf("Unable to load flags from current directory")

		return nil
	}

	// Validate and merge.
	validationErrors := m.validator.Validate(result.Flags)
	if err := m.handleValidationErrors(validationErrors, ErrFlagsValidationFailed, "flags configuration"); err != nil {
		return err
	}

	if err := m.merger.MergeIntoViper(result.Flags, command); err != nil {
		return fmt.Errorf("failed to merge flags from current directory: %w", err)
	}

	logrus.Debugf("Successfully loaded and merged flags from current directory for command %s", command)

	return nil
}

// GetConfigPathFromViper gets the configuration path from viper, with fallback to default.
func GetConfigPathFromViper() string {
	configPath := viper.GetString("config")
	if configPath == "" {
		configPath = "furyctl.yaml"
	}

	return configPath
}

// isCriticalError determines if an error should cause the flags loading to fail
// rather than just log a warning.
func isCriticalError(err error) bool {
	// Configuration file not found is not critical (expected in many cases).
	if errors.Is(err, ErrConfigurationFileNotFound) || errors.Is(err, ErrNoFuryctlConfigFileFound) {
		return false
	}

	// Check if this is a dynamic value parsing error.
	if errors.Is(err, parser.ErrCannotParseDynamicValue) {
		errMsg := err.Error()

		// File-related errors are critical.
		if strings.Contains(errMsg, "no such file") ||
			strings.Contains(errMsg, "cannot find") ||
			strings.Contains(errMsg, "file not found") ||
			strings.Contains(errMsg, "permission denied") {
			return true
		}

		// Environment variable errors are also critical.
		// Error format: "cannot parse dynamic value: \"VARIABLE_NAME\" is empty".
		if strings.Contains(errMsg, "is empty") {
			return true
		}

		// HTTP/HTTPS download errors are critical.
		if strings.Contains(errMsg, "failed to download") ||
			strings.Contains(errMsg, "http error") {
			return true
		}
	}

	// YAML parsing errors and other processing errors are critical.
	if strings.Contains(err.Error(), "failed to parse configuration file") ||
		strings.Contains(err.Error(), "failed to process dynamic values") {
		return true
	}

	return false
}

// handleValidationErrors processes validation errors by separating fatal errors from warnings,
// logging them appropriately, and returning early for fatal errors.
func (*Manager) handleValidationErrors(validationErrors []ValidationError, fatalError error, context string) error {
	if len(validationErrors) == 0 {
		return nil
	}

	// Separate fatal errors from warnings.
	var fatalErrors []ValidationError

	var warnings []ValidationError

	for _, valErr := range validationErrors {
		if valErr.Severity == ValidationSeverityFatal {
			fatalErrors = append(fatalErrors, valErr)
		} else {
			warnings = append(warnings, valErr)
		}
	}

	// Return immediately if there are fatal errors.
	if len(fatalErrors) > 0 {
		logrus.Errorf("Found %d fatal validation errors in %s:", len(fatalErrors), context)

		for _, fatalErr := range fatalErrors {
			logrus.Errorf("  %v", fatalErr)
		}

		return fmt.Errorf("%w with %d fatal errors", fatalError, len(fatalErrors))
	}

	// Log warnings but continue execution.
	if len(warnings) > 0 {
		logrus.Warnf("Found %d validation warnings in %s:", len(warnings), context)

		for _, warning := range warnings {
			logrus.Warnf("  %v", warning)
		}
	}

	return nil
}

// LoadAndMergeCommandFlags loads and merges flags for a specific command with proper error handling.
func LoadAndMergeCommandFlags(command string) error {
	configPath := GetConfigPathFromViper()
	flagsManager := NewManager(filepath.Dir(configPath))

	if err := flagsManager.LoadAndMergeFlags(configPath, command); err != nil {
		// Critical errors (like missing environment variables) should stop execution.
		return fmt.Errorf("failed to load flags from configuration: %w", err)
	}

	return nil
}

// LoadAndMergeGlobalFlagsFromArgs loads global flags from command line --config argument.
// This is called early in PersistentPreRun before log file creation.
func LoadAndMergeGlobalFlagsFromArgs() error {
	flagsManager := NewManager(".")

	// Parse command line args directly since individual command flags haven't been bound to viper yet.
	var configPath string

	args := os.Args

	for i, arg := range args {
		if arg == "--config" || arg == "-c" {
			if i+1 < len(args) {
				configPath = args[i+1]

				break
			}
		} else if strings.HasPrefix(arg, "--config=") {
			configPath = strings.TrimPrefix(arg, "--config=")

			break
		}
	}

	if configPath != "" {
		if err := flagsManager.LoadAndMergeGlobalFlags(configPath); err != nil {
			// Critical flag expansion errors should be fatal before log file creation
			// to prevent directory creation with unexpanded dynamic values.
			if strings.Contains(err.Error(), "cannot parse dynamic value") ||
				strings.Contains(err.Error(), "is empty") ||
				strings.Contains(err.Error(), "failed to process dynamic values") {
				return fmt.Errorf("critical flag expansion error in %s: %w", configPath, err)
			}

			logrus.Debugf("Failed to load global flags from %s: %v", configPath, err)
		}
	}

	if err := flagsManager.TryLoadFromCurrentDirectory("global"); err != nil {
		// Continue execution - global flags loading is optional.
		logrus.Debugf("Failed to load global flags from current directory: %v", err)
	}

	return nil
}
