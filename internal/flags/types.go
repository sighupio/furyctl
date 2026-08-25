// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flags

// FlagsConfig holds the sections of the `flags` field of furyctl.yaml, keyed by the command name.
//
//nolint:revive // FlagsConfig name is intentionally explicit for external API clarity
type FlagsConfig map[string]map[string]any

// SupportedFlags holds the flags that furyctl supports in each section, keyed by the command name.
// A section that is absent from this map is not a section that furyctl supports.
type SupportedFlags map[string]map[string]FlagType

// FlagType represents the type of a flag value.
type FlagType string

const (
	FlagTypeString      FlagType = "string"
	FlagTypeBool        FlagType = "bool"
	FlagTypeInt         FlagType = "int"
	FlagTypeStringSlice FlagType = "stringSlice"
	FlagTypeDuration    FlagType = "duration"

	// ValidationSeverityFatal indicates a critical error that should stop execution.
	ValidationSeverityFatal ValidationSeverity = "fatal"
	// ValidationSeverityWarning indicates a non-critical error that should log a warning.
	ValidationSeverityWarning ValidationSeverity = "warning"
)

// ValidationSeverity represents the severity level of a validation error.
type ValidationSeverity string

// ConfigWithFlags represents a furyctl configuration that may contain flags.
type ConfigWithFlags struct {
	APIVersion string         `yaml:"apiVersion"`
	Kind       string         `yaml:"kind"`
	Metadata   map[string]any `yaml:"metadata"`
	Spec       map[string]any `yaml:"spec"`
	Flags      FlagsConfig    `yaml:"flags,omitempty"`
}

// LoadResult contains the result of loading and processing flags.
type LoadResult struct {
	ConfigPath string
	Flags      FlagsConfig
	Errors     []error
}

// ValidationError represents an error that occurred during flag validation.
type ValidationError struct {
	Command  string
	Flag     string
	Value    any
	Reason   string
	Severity ValidationSeverity
}

func (e ValidationError) Error() string {
	severityStr := string(e.Severity)
	if e.Flag != "" {
		return severityStr + " validation error for " + e.Command + "." + e.Flag + ": " + e.Reason
	}

	return severityStr + " validation error for " + e.Command + ": " + e.Reason
}

// GetSupportedFlags returns the complete mapping of supported flags for all commands.
func GetSupportedFlags() SupportedFlags {
	return SupportedFlags{
		CommandGlobal: {
			"debug":            FlagTypeBool,
			"disableAnalytics": FlagTypeBool,
			"noTty":            FlagTypeBool,
			"workdir":          FlagTypeString,
			"outdir":           FlagTypeString,
			"log":              FlagTypeString,
			"gitProtocol":      FlagTypeString,
		},
		CommandApply: {
			"phase":                  FlagTypeString,
			"startFrom":              FlagTypeString,
			"distroLocation":         FlagTypeString,
			"distroPatches":          FlagTypeString,
			"binPath":                FlagTypeString,
			"skipNodesUpgrade":       FlagTypeBool,
			"skipDepsDownload":       FlagTypeBool,
			"skipDepsValidation":     FlagTypeBool,
			"dryRun":                 FlagTypeBool,
			"vpnAutoConnect":         FlagTypeBool,
			"skipVpnConfirmation":    FlagTypeBool,
			"force":                  FlagTypeStringSlice,
			"postApplyPhases":        FlagTypeStringSlice,
			"timeout":                FlagTypeInt,
			"podRunningCheckTimeout": FlagTypeInt,
			"upgrade":                FlagTypeBool,
			"upgradePathLocation":    FlagTypeString,
			"upgradeNode":            FlagTypeString,
			"airgapBundle":           FlagTypeString,
			"forceExtract":           FlagTypeBool,
		},
		CommandDelete: {
			"phase":               FlagTypeString,
			"startFrom":           FlagTypeString,
			"distroLocation":      FlagTypeString,
			"distroPatches":       FlagTypeString,
			"binPath":             FlagTypeString,
			"dryRun":              FlagTypeBool,
			"skipVpnConfirmation": FlagTypeBool,
			"autoApprove":         FlagTypeBool,
			"airgapBundle":        FlagTypeString,
			"forceExtract":        FlagTypeBool,
		},
		CommandCreate: {
			"name":         FlagTypeString,
			"version":      FlagTypeString,
			"provider":     FlagTypeString,
			"path":         FlagTypeString,
			"etcd":         FlagTypeBool,
			"controlplane": FlagTypeBool,
		},
		CommandGet: {
			"binPath":            FlagTypeString,
			"distroLocation":     FlagTypeString,
			"skipDepsDownload":   FlagTypeBool,
			"skipDepsValidation": FlagTypeBool,
			"airgapBundle":       FlagTypeString,
			"forceExtract":       FlagTypeBool,
		},
		CommandDiff: {
			"phase":               FlagTypeString,
			"distroLocation":      FlagTypeString,
			"distroPatches":       FlagTypeString,
			"binPath":             FlagTypeString,
			"upgradePathLocation": FlagTypeString,
			"airgapBundle":        FlagTypeString,
			"forceExtract":        FlagTypeBool,
		},
		CommandValidate: {
			"distroLocation": FlagTypeString,
			"distroPatches":  FlagTypeString,
			"binPath":        FlagTypeString,
		},
		CommandDownload: {
			"binPath":        FlagTypeString,
			"distroLocation": FlagTypeString,
			"distroPatches":  FlagTypeString,
			"bundleOutput":   FlagTypeString,
		},
		CommandConnect: {
			"profile": FlagTypeString,
		},
		CommandRenew: {
			"airgapBundle":       FlagTypeString,
			"forceExtract":       FlagTypeBool,
			"binPath":            FlagTypeString,
			"distroLocation":     FlagTypeString,
			"skipDepsDownload":   FlagTypeBool,
			"skipDepsValidation": FlagTypeBool,
		},
		CommandDump: {
			"distroLocation": FlagTypeString,
			"distroPatches":  FlagTypeString,
			"dryRun":         FlagTypeBool,
			"noOverwrite":    FlagTypeBool,
			"skipValidation": FlagTypeBool,
		},
	}
}
