// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flags

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	parserx "github.com/sighupio/furyctl/internal/parser"
	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

// Static error definitions for linting compliance.
var (
	ErrConfigurationFileNotFound = errors.New("configuration file not found")
	ErrNoFuryctlConfigFileFound  = errors.New("no furyctl configuration file found in directory")
)

// Loader handles loading flags configuration from furyctl.yaml files.
type Loader struct {
	configParser *parserx.ConfigParser
}

// NewLoader creates a new flags loader with the given base directory.
func NewLoader(baseDir string) *Loader {
	return &Loader{
		configParser: parserx.NewConfigParser(baseDir),
	}
}

// LoadFromFile loads flags configuration from the specified furyctl.yaml file.
func (l *Loader) LoadFromFile(configPath string) (*LoadResult, error) {
	result := &LoadResult{
		ConfigPath: configPath,
		Flags:      nil,
		Errors:     []error{},
	}

	// Ensure the config file exists.
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		result.Errors = append(result.Errors, fmt.Errorf("%w: %s", ErrConfigurationFileNotFound, configPath))

		return result, nil
	}

	// Load the configuration file.
	config, err := yamlx.FromFileV3[ConfigWithFlags](configPath)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Errorf("failed to parse configuration file: %w", err))

		return result, nil
	}

	// If no flags section exists, return empty result.
	if config.Flags == nil {
		return result, nil
	}

	// Process dynamic values in the flags configuration.
	processedFlags, err := l.processDynamicValues(config.Flags)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Errorf("failed to process dynamic values: %w", err))

		return result, nil
	}

	result.Flags = processedFlags

	return result, nil
}

// LoadFromDirectory tries to find and load flags from a furyctl.yaml file in the given directory..
func (l *Loader) LoadFromDirectory(dir string) (*LoadResult, error) {
	// Common configuration file names to try.
	configNames := []string{"furyctl.yaml", "furyctl.yml"}

	for _, name := range configNames {
		configPath := filepath.Join(dir, name)
		if _, err := os.Stat(configPath); err == nil {
			return l.LoadFromFile(configPath)
		}
	}

	// No configuration file found.
	result := &LoadResult{
		ConfigPath: "",
		Flags:      nil,
		Errors:     []error{fmt.Errorf("%w: %s", ErrNoFuryctlConfigFileFound, dir)},
	}

	return result, nil
}

// processDynamicValues processes dynamic values like {env://VAR} and {file://path} in the flags configuration.
func (l *Loader) processDynamicValues(flags *FlagsConfig) (*FlagsConfig, error) {
	processed := &FlagsConfig{}

	if err := l.processField(flags.Global, &processed.Global, "global"); err != nil {
		return nil, err
	}

	if err := l.processField(flags.Apply, &processed.Apply, "apply"); err != nil {
		return nil, err
	}

	if err := l.processField(flags.Delete, &processed.Delete, "delete"); err != nil {
		return nil, err
	}

	if err := l.processField(flags.Create, &processed.Create, "create"); err != nil {
		return nil, err
	}

	if err := l.processField(flags.Get, &processed.Get, "get"); err != nil {
		return nil, err
	}

	if err := l.processField(flags.Diff, &processed.Diff, "diff"); err != nil {
		return nil, err
	}

	if err := l.processField(flags.Tools, &processed.Tools, "tools"); err != nil {
		return nil, err
	}

	if err := l.processField(flags.Validate, &processed.Validate, "validate"); err != nil {
		return nil, err
	}

	if err := l.processField(flags.Download, &processed.Download, "download"); err != nil {
		return nil, err
	}

	if err := l.processField(flags.Connect, &processed.Connect, "connect"); err != nil {
		return nil, err
	}

	if err := l.processField(flags.Renew, &processed.Renew, "renew"); err != nil {
		return nil, err
	}

	if err := l.processField(flags.Dump, &processed.Dump, "dump"); err != nil {
		return nil, err
	}

	return processed, nil
}

// processCommandFlags processes dynamic values in a single command's flags map.
func (l *Loader) processCommandFlags(flagsMap map[string]any) (map[string]any, error) {
	processed := make(map[string]any)

	for key, value := range flagsMap {
		processedValue, err := l.configParser.ParseDynamicValue(value)
		if err != nil {
			return nil, fmt.Errorf("error processing flag %s: %w", key, err)
		}

		processed[key] = processedValue
	}

	return processed, nil
}

func (l *Loader) processField(src map[string]any, dst *map[string]any, name string) error {
	if src == nil {
		return nil
	}

	var err error

	*dst, err = l.processCommandFlags(src)
	if err != nil {
		return fmt.Errorf("error processing %s flags: %w", name, err)
	}

	return nil
}
