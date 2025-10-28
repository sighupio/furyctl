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
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/furyctl/internal/analytics"
	"github.com/sighupio/furyctl/internal/apis"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/flags"
	"github.com/sighupio/furyctl/internal/parser"
	"github.com/sighupio/furyctl/internal/schema/santhosh"
	iox "github.com/sighupio/furyctl/internal/x/io"
	dist "github.com/sighupio/furyctl/pkg/distribution"
	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

// Static error definitions for linting compliance.
var (
	ErrFlagsMustBeObject            = errors.New("flags section must be an object")
	ErrUnsupportedFlagsCommand      = errors.New("unsupported flags command")
	ErrFlagsValidationFailed        = errors.New("flags validation failed")
	ErrExpandedConfigurationNotAMap = errors.New("expanded configuration is not a map[string]any")
	ErrReadingSpec                  = errors.New("error reading spec from kfd.yaml")
	ErrReadingToolsConfiguration    = errors.New("error reading spec.toolsConfiguration from kfd.yaml")
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

	// Check if the schema supports flags field.
	schemaSupportsFlags := checkSchemaSupportsFlags(schemaPath)

	// Choose validation path based on schema capabilities.
	if schemaSupportsFlags {
		// New path: schema knows about flags, validate with flags included.
		expandedConf, err := expandDynamicValues(rawConf, filepath.Dir(path))
		if err != nil {
			return fmt.Errorf("error expanding dynamic values: %w", err)
		}

		// Validate configuration with flags included.
		if err = schema.Validate(expandedConf); err != nil {
			return fmt.Errorf("error while validating against schema: %w", err)
		}
	} else {
		// Fallback path: old schema, strip flags before validation (current behavior).
		cleanConf := createCleanConfigForSchemaValidation(rawConf)

		// Expand dynamic values before schema validation.
		expandedConf, err := expandDynamicValues(cleanConf, filepath.Dir(path))
		if err != nil {
			return fmt.Errorf("error expanding dynamic values: %w", err)
		}

		// Validate expanded configuration against fury-distribution schema.
		if err = schema.Validate(expandedConf); err != nil {
			return fmt.Errorf("error while validating against schema: %w", err)
		}
	}

	// Run additional schema validation rules.
	esv := apis.NewExtraSchemaValidatorFactory(miniConf.APIVersion, miniConf.Kind)
	if err = esv.Validate(path); err != nil {
		return fmt.Errorf("error while validating against extra schema rules: %w", err)
	}

	// Validate configuration between kfd.yaml and furyctl.yaml files for Terraform/OpenTofu.
	err = validateToolsConfiguration(repoPath, rawConf)

	return err
}

// checkSchemaSupportsFlags determines if the schema includes support for the flags field.
// This allows furyctl to work with both old schemas (without flags) and new schemas (with flags).
func checkSchemaSupportsFlags(schemaPath string) bool {
	// Simple check: does the schema file reference spec-flags.json?
	content, err := os.ReadFile(schemaPath)
	if err != nil {
		// On error, assume old schema to maintain backward compatibility.
		logrus.Debugf("Could not read schema file to check flags support: %v", err)

		return false
	}

	// Check if the schema references the flags specification.
	hasFlags := strings.Contains(string(content), "spec-flags.json")

	if hasFlags {
		logrus.Debug("Schema supports flags field, validating with flags included")
	} else {
		logrus.Debug("Schema does not support flags field, using fallback validation")
	}

	return hasFlags
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

// expandDynamicValues recursively expands dynamic values in the configuration
// before schema validation.
func expandDynamicValues(conf map[string]any, baseDir string) (map[string]any, error) {
	result, err := expandDynamicValuesRecursive(conf, parser.NewConfigParser(baseDir))
	if err != nil {
		return nil, err
	}

	// Type assert the result back to map[string]any.
	expandedConf, ok := result.(map[string]any)
	if !ok {
		return nil, ErrExpandedConfigurationNotAMap
	}

	return expandedConf, nil
}

// expandDynamicValuesRecursive recursively processes the configuration map to expand dynamic values.
func expandDynamicValuesRecursive(value any, configParser *parser.ConfigParser) (any, error) {
	switch v := value.(type) {
	case map[string]any:
		result := make(map[string]any)

		for key, val := range v {
			expandedVal, err := expandDynamicValuesRecursive(val, configParser)
			if err != nil {
				return nil, fmt.Errorf("error expanding value for key %s: %w", key, err)
			}

			result[key] = expandedVal
		}

		return result, nil

	case []any:
		result := make([]any, len(v))

		for i, val := range v {
			expandedVal, err := expandDynamicValuesRecursive(val, configParser)
			if err != nil {
				return nil, fmt.Errorf("error expanding array element %d: %w", i, err)
			}

			result[i] = expandedVal
		}

		return result, nil

	case string:
		// Check if this string contains dynamic value patterns.
		if containsDynamicPattern(v) {
			expandedVal, err := configParser.ParseDynamicValue(v)
			if err != nil {
				return nil, fmt.Errorf("error parsing dynamic value: %w", err)
			}

			return expandedVal, nil
		}

		return v, nil

	default:
		// For other types (bool, int, float, etc.), return as-is.
		return value, nil
	}
}

const (
	envPrefixLen   = 6 // "env://"
	filePrefixLen  = 7 // "file://"
	httpPrefixLen  = 8 // "http://"
	httpsPrefixLen = 9 // "https://"
	pathPrefixLen  = 8 // "path://"
)

// containsDynamicPattern checks if a string contains dynamic value patterns like {env://}, {file://}, etc.
func containsDynamicPattern(s string) bool {
	// Simple check for dynamic value patterns.
	return len(s) > 0 && s[0] == '{' && ((len(s) > envPrefixLen && s[1:envPrefixLen+1] == "env://") ||
		(len(s) > filePrefixLen && s[1:filePrefixLen+1] == "file://") ||
		(len(s) > httpPrefixLen && s[1:httpPrefixLen+1] == "http://") ||
		(len(s) > httpsPrefixLen && s[1:httpsPrefixLen+1] == "https://") ||
		(len(s) > pathPrefixLen && s[1:pathPrefixLen+1] == "path://"))
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
		// Separate fatal errors from warnings.
		var fatalErrors []flags.ValidationError

		var warnings []flags.ValidationError

		for _, err := range validationErrors {
			if err.Severity == flags.ValidationSeverityFatal {
				fatalErrors = append(fatalErrors, err)
			} else {
				warnings = append(warnings, err)
			}
		}

		// Log warnings but don't fail validation.
		if len(warnings) > 0 {
			logrus.Warnf("Found %d validation warnings in flags configuration:", len(warnings))

			for _, warning := range warnings {
				logrus.Warnf("  %v", warning)
			}
		}

		// Only fail validation on fatal errors.
		if len(fatalErrors) > 0 {
			return fmt.Errorf("%w: %v", ErrFlagsValidationFailed, fatalErrors)
		}
	}

	return nil
}

// validateToolsConfiguration checks that the tool configured in furyctl.yaml
// is available in kfd.yaml file.
func validateToolsConfiguration(repoPath string, furyctlConf map[string]any) error {
	kfdPath := filepath.Join(repoPath, "kfd.yaml")

	kfdFile, err := yamlx.FromFileV3[config.KFD](kfdPath)
	if err != nil {
		return fmt.Errorf("%w: %w", dist.ErrYamlUnmarshalFile, err)
	}

	spec, exists := furyctlConf["spec"].(map[string]any)
	if !exists {
		return fmt.Errorf("%w: %w", ErrReadingSpec, err)
	}

	toolsConfig, exists := spec["toolsConfiguration"].(map[string]any)
	if !exists {
		return fmt.Errorf("%w: %w", ErrReadingToolsConfiguration, err)
	}

	_, hasTerraformConfig := toolsConfig["terraform"]

	if hasTerraformConfig && kfdFile.Tools.Common.OpenTofu.Version != "" {
		logrus.Warn("'spec.toolsConfiguration.terraform' is deprecated, " +
			"it will be removed in future versions. " +
			"Please use 'spec.toolsConfiguration.opentofu' instead")
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
