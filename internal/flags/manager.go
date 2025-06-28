// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flags

import (
	"fmt"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
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
	// Resolve absolute path for the config
	absConfigPath, err := filepath.Abs(configPath)
	if err != nil {
		return fmt.Errorf("failed to resolve config path: %w", err)
	}

	// Load flags from configuration file
	result, err := m.loader.LoadFromFile(absConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load flags: %w", err)
	}

	// Check for loading errors
	if len(result.Errors) > 0 {
		logrus.Debugf("Flags loading completed with %d errors", len(result.Errors))
		for _, loadErr := range result.Errors {
			logrus.Debugf("Flags loading error: %v", loadErr)
		}
		// If no flags were loaded due to errors, just return (no flags to merge)
		if result.Flags == nil {
			return nil
		}
	}

	// If no flags configuration found, nothing to merge
	if result.Flags == nil {
		logrus.Debugf("No flags configuration found in %s", configPath)
		return nil
	}

	// Validate the flags configuration
	validationErrors := m.validator.Validate(result.Flags)
	if len(validationErrors) > 0 {
		logrus.Warnf("Found %d validation errors in flags configuration:", len(validationErrors))
		for _, valErr := range validationErrors {
			logrus.Warnf("  %v", valErr)
		}
		// Continue with merging despite validation errors (warnings only)
	}

	// Merge flags into viper with lowest priority
	if err := m.merger.MergeIntoViper(result.Flags, command); err != nil {
		return fmt.Errorf("failed to merge flags: %w", err)
	}

	logrus.Debugf("Successfully loaded and merged flags from %s for command %s", configPath, command)
	return nil
}

// LoadAndMergeGlobalFlags loads and merges only global flags.
func (m *Manager) LoadAndMergeGlobalFlags(configPath string) error {
	// Resolve absolute path for the config
	absConfigPath, err := filepath.Abs(configPath)
	if err != nil {
		return fmt.Errorf("failed to resolve config path: %w", err)
	}

	// Load flags from configuration file
	result, err := m.loader.LoadFromFile(absConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load global flags: %w", err)
	}

	// Check for loading errors
	if len(result.Errors) > 0 {
		logrus.Debugf("Global flags loading completed with %d errors", len(result.Errors))
		for _, loadErr := range result.Errors {
			logrus.Debugf("Global flags loading error: %v", loadErr)
		}
		// If no flags were loaded due to errors, just return
		if result.Flags == nil {
			return nil
		}
	}

	// If no flags configuration found, nothing to merge
	if result.Flags == nil {
		logrus.Debugf("No global flags configuration found in %s", configPath)
		return nil
	}

	// Validate only global flags
	if result.Flags.Global != nil {
		validationErrors := m.validator.validateCommandFlags(result.Flags.Global, "global")
		if len(validationErrors) > 0 {
			logrus.Warnf("Found %d validation errors in global flags configuration:", len(validationErrors))
			for _, valErr := range validationErrors {
				logrus.Warnf("  %v", valErr)
			}
		}
	}

	// Merge only global flags
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
		// This is expected if no config file exists
		logrus.Debugf("No configuration file found in current directory: %v", err)
		return nil
	}

	if len(result.Errors) > 0 || result.Flags == nil {
		logrus.Debugf("Unable to load flags from current directory")
		return nil
	}

	// Validate and merge
	validationErrors := m.validator.Validate(result.Flags)
	if len(validationErrors) > 0 {
		logrus.Warnf("Found %d validation errors in flags configuration:", len(validationErrors))
		for _, valErr := range validationErrors {
			logrus.Warnf("  %v", valErr)
		}
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
