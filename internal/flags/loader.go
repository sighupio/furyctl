// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flags

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sighupio/furyctl/internal/parser"
	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

// Static error definitions for linting compliance.
var (
	ErrConfigurationFileNotFound = errors.New("configuration file not found")
	ErrNoFuryctlConfigFileFound  = errors.New("no furyctl configuration file found in directory")
)

// Loader handles loading flags configuration from furyctl.yaml files.
type Loader struct {
	configParser *parser.ConfigParser
}

// NewLoader creates a new flags loader with the given base directory.
func NewLoader(baseDir string) *Loader {
	return &Loader{
		configParser: parser.NewConfigParser(baseDir),
	}
}

// LoadFromFile loads flags configuration from the specified furyctl.yaml file.
func (l *Loader) LoadFromFile(configPath string) (*LoadResult, error) {
	// Ensure the config file exists.
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("%w: %s", ErrConfigurationFileNotFound, configPath)
	}

	// Load the configuration file.
	config, err := yamlx.FromFileV3[ConfigWithFlags](configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse configuration file: %w", err)
	}

	// If no flags section exists, return empty result.
	if config.Flags == nil {
		return &LoadResult{
			ConfigPath: configPath,
			Flags:      nil,
		}, nil
	}

	// Process dynamic values in the flags configuration.
	processedFlags, err := l.processDynamicValues(config.Flags)
	if err != nil {
		return nil, fmt.Errorf("failed to process dynamic values: %w", err)
	}

	return &LoadResult{
		ConfigPath: configPath,
		Flags:      processedFlags,
	}, nil
}

// processDynamicValues processes dynamic values like {env://VAR} and {file://path} in the flags configuration.
func (l *Loader) processDynamicValues(flags *FlagsConfig) (*FlagsConfig, error) {
	processed := &FlagsConfig{}

	var err error

	// Process each command's flags.
	if flags.Global != nil {
		processed.Global, err = l.processCommandFlags(flags.Global)
		if err != nil {
			return nil, fmt.Errorf("error processing global flags: %w", err)
		}
	}

	if flags.Apply != nil {
		processed.Apply, err = l.processCommandFlags(flags.Apply)
		if err != nil {
			return nil, fmt.Errorf("error processing apply flags: %w", err)
		}
	}

	if flags.Delete != nil {
		processed.Delete, err = l.processCommandFlags(flags.Delete)
		if err != nil {
			return nil, fmt.Errorf("error processing delete flags: %w", err)
		}
	}

	if flags.Create != nil {
		processed.Create, err = l.processCommandFlags(flags.Create)
		if err != nil {
			return nil, fmt.Errorf("error processing create flags: %w", err)
		}
	}

	if flags.Get != nil {
		processed.Get, err = l.processCommandFlags(flags.Get)
		if err != nil {
			return nil, fmt.Errorf("error processing get flags: %w", err)
		}
	}

	if flags.Diff != nil {
		processed.Diff, err = l.processCommandFlags(flags.Diff)
		if err != nil {
			return nil, fmt.Errorf("error processing diff flags: %w", err)
		}
	}

	if flags.Tools != nil {
		processed.Tools, err = l.processCommandFlags(flags.Tools)
		if err != nil {
			return nil, fmt.Errorf("error processing tools flags: %w", err)
		}
	}

	if flags.Validate != nil {
		processed.Validate, err = l.processCommandFlags(flags.Validate)
		if err != nil {
			return nil, fmt.Errorf("error processing validate flags: %w", err)
		}
	}

	if flags.Download != nil {
		processed.Download, err = l.processCommandFlags(flags.Download)
		if err != nil {
			return nil, fmt.Errorf("error processing download flags: %w", err)
		}
	}

	if flags.Connect != nil {
		processed.Connect, err = l.processCommandFlags(flags.Connect)
		if err != nil {
			return nil, fmt.Errorf("error processing connect flags: %w", err)
		}
	}

	if flags.Renew != nil {
		processed.Renew, err = l.processCommandFlags(flags.Renew)
		if err != nil {
			return nil, fmt.Errorf("error processing renew flags: %w", err)
		}
	}

	if flags.Dump != nil {
		processed.Dump, err = l.processCommandFlags(flags.Dump)
		if err != nil {
			return nil, fmt.Errorf("error processing dump flags: %w", err)
		}
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
	return nil, fmt.Errorf("%w: %s", ErrNoFuryctlConfigFileFound, dir)
}
