// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package config

import (
	"errors"
	"fmt"
	"html/template"
	"os"
	"path/filepath"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/furyctl/internal/analytics"
	"github.com/sighupio/furyctl/internal/apis"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/flags"
	"github.com/sighupio/furyctl/internal/schema/santhosh"
	iox "github.com/sighupio/furyctl/internal/x/io"
	dist "github.com/sighupio/furyctl/pkg/distribution"
	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

// Static error definitions for linting compliance.
var (
	ErrFlagsMustBeObject       = errors.New("flags section must be an object")
	ErrUnsupportedFlagsCommand = errors.New("unsupported flags command")
	ErrFlagsValidationFailed   = errors.New("flags validation failed")
)

func Create(
	res dist.DownloadResult,
	furyctlPath string,
	cmdEvent analytics.Event,
	tracker *analytics.Tracker,
	data map[string]string,
) (*os.File, error) {
	tplPath, err := distribution.GetConfigTemplatePath(res.RepoPath, res.MinimalConf)
	if err != nil {
		return nil, fmt.Errorf("error getting schema path: %w", err)
	}

	tplContent, err := os.ReadFile(tplPath)
	if err != nil {
		cmdEvent.AddErrorMessage(err)
		tracker.Track(cmdEvent)

		return nil, fmt.Errorf("error reading furyctl yaml template: %w", err)
	}

	tmpl, err := template.New("furyctl.yaml").Parse(string(tplContent))
	if err != nil {
		cmdEvent.AddErrorMessage(err)
		tracker.Track(cmdEvent)

		return nil, fmt.Errorf("error parsing furyctl yaml template: %w", err)
	}

	out, err := createNewEmptyConfigFile(furyctlPath)
	if err != nil {
		cmdEvent.AddErrorMessage(err)
		tracker.Track(cmdEvent)

		return nil, err
	}

	if err := tmpl.Execute(out, data); err != nil {
		cmdEvent.AddErrorMessage(err)
		tracker.Track(cmdEvent)

		return nil, fmt.Errorf("error executing furyctl yaml template: %w", err)
	}

	return out, nil
}

func createNewEmptyConfigFile(path string) (*os.File, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("error getting absolute path: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(absPath), iox.FullPermAccess); err != nil {
		return nil, fmt.Errorf("error creating directory: %w", err)
	}

	out, err := os.Create(absPath)
	if err != nil {
		return nil, fmt.Errorf("error creating file: %w", err)
	}

	return out, nil
}

// Validate the furyctl.yaml file using preprocessing approach to handle flags section.
func Validate(path, repoPath string) error {
	miniConf, err := loadFromFile(path)
	if err != nil {
		return err
	}

	schemaPath, err := distribution.GetPublicSchemaPath(repoPath, miniConf)
	if err != nil {
		return fmt.Errorf("error getting schema path: %w", err)
	}

	schema, err := santhosh.LoadSchema(schemaPath)
	if err != nil {
		return fmt.Errorf("error loading schema: %w", err)
	}

	// Load raw configuration as map for preprocessing.
	rawConf, err := yamlx.FromFileV3[map[string]any](path)
	if err != nil {
		return err
	}

	// Extract and validate flags section separately if it exists.
	if flagsSection, exists := rawConf["flags"]; exists {
		if err := validateFlagsSection(flagsSection); err != nil {
			return fmt.Errorf("error validating flags section: %w", err)
		}
	}

	// Create clean configuration without flags for schema validation.
	cleanConf := createCleanConfigForSchemaValidation(rawConf)

	// Validate clean configuration against fury-distribution schema.
	if err = schema.Validate(cleanConf); err != nil {
		return fmt.Errorf("error while validating against schema: %w", err)
	}

	// Run additional schema validation rules.
	esv := apis.NewExtraSchemaValidatorFactory(miniConf.APIVersion, miniConf.Kind)
	if err = esv.Validate(path); err != nil {
		return fmt.Errorf("error while validating against extra schema rules: %w", err)
	}

	return nil
}

// createCleanConfigForSchemaValidation removes furyctl-specific sections that shouldn't be validated
// against fury-distribution schemas.
func createCleanConfigForSchemaValidation(rawConf map[string]any) map[string]any {
	cleanConf := make(map[string]any)

	// Copy all sections except furyctl-specific ones.
	for key, value := range rawConf {
		switch key {
		case "flags":
			// Skip flags section - it's furyctl-specific.
			continue

		default:
			// Copy all other sections for schema validation.
			cleanConf[key] = value
		}
	}

	return cleanConf
}

// validateFlagsSection validates the flags section using furyctl-specific validation rules.
func validateFlagsSection(flagsSection any) error {
	// Convert to FlagsConfig type for validation.
	flagsMap, ok := flagsSection.(map[string]any)
	if !ok {
		return ErrFlagsMustBeObject
	}

	// Convert to internal flags structure for validation.
	flagsConfig := &flags.FlagsConfig{}

	// Extract and validate each command section.
	for command, commandFlags := range flagsMap {
		commandFlagsMap, ok := commandFlags.(map[string]any)
		if !ok {
			return fmt.Errorf("%w: flags.%s must be an object", ErrFlagsMustBeObject, command)
		}

		// Set the command flags in the appropriate section.
		switch command {
		case "global":
			flagsConfig.Global = commandFlagsMap

		case "apply":
			flagsConfig.Apply = commandFlagsMap

		case "delete":
			flagsConfig.Delete = commandFlagsMap

		case "create":
			flagsConfig.Create = commandFlagsMap

		case "get":
			flagsConfig.Get = commandFlagsMap

		case "diff":
			flagsConfig.Diff = commandFlagsMap

		case "tools":
			flagsConfig.Tools = commandFlagsMap

		default:
			return fmt.Errorf("%w: %s", ErrUnsupportedFlagsCommand, command)
		}
	}

	// Validate flags using the flags package validator.
	validator := flags.NewValidator()
	validationErrors := validator.Validate(flagsConfig)

	if len(validationErrors) > 0 {
		return fmt.Errorf("%w: %v", ErrFlagsValidationFailed, validationErrors)
	}

	return nil
}

func loadFromFile(path string) (config.Furyctl, error) {
	conf, err := yamlx.FromFileV3[config.Furyctl](path)
	if err != nil {
		return config.Furyctl{}, err
	}

	if err := config.NewValidator().Struct(conf); err != nil {
		return config.Furyctl{}, fmt.Errorf("%w: %v", dist.ErrValidateConfig, err)
	}

	return conf, err
}
