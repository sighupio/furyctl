// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flags

// FlagsConfig represents the flags section in a furyctl.yaml file.
//
//nolint:revive // FlagsConfig name is intentionally explicit for external API clarity
type FlagsConfig struct {
	Global   map[string]any `yaml:"global,omitempty"`
	Apply    map[string]any `yaml:"apply,omitempty"`
	Delete   map[string]any `yaml:"delete,omitempty"`
	Create   map[string]any `yaml:"create,omitempty"`
	Get      map[string]any `yaml:"get,omitempty"`
	Diff     map[string]any `yaml:"diff,omitempty"`
	Tools    map[string]any `yaml:"tools,omitempty"`
	Validate map[string]any `yaml:"validate,omitempty"`
	Download map[string]any `yaml:"download,omitempty"`
	Connect  map[string]any `yaml:"connect,omitempty"`
	Renew    map[string]any `yaml:"renew,omitempty"`
	Dump     map[string]any `yaml:"dump,omitempty"`
}

// SupportedFlags defines the mapping between flag names and their expected types
// This helps with validation and type conversion.
type SupportedFlags struct {
	Global   map[string]FlagInfo
	Apply    map[string]FlagInfo
	Delete   map[string]FlagInfo
	Create   map[string]FlagInfo
	Get      map[string]FlagInfo
	Diff     map[string]FlagInfo
	Tools    map[string]FlagInfo
	Validate map[string]FlagInfo
	Download map[string]FlagInfo
	Connect  map[string]FlagInfo
	Renew    map[string]FlagInfo
	Dump     map[string]FlagInfo
}

// FlagInfo contains metadata about a supported flag.
type FlagInfo struct {
	Type         FlagType
	DefaultValue any
	Description  string
}

// FlagType represents the type of a flag value.
type FlagType string

const (
	FlagTypeString      FlagType = "string"
	FlagTypeBool        FlagType = "bool"
	FlagTypeInt         FlagType = "int"
	FlagTypeStringSlice FlagType = "stringSlice"
	FlagTypeDuration    FlagType = "duration"

	// Default timeout values.
	DefaultTimeoutSeconds         = 3600
	DefaultPodRunningCheckTimeout = 300

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
	Flags      *FlagsConfig   `yaml:"flags,omitempty"`
}

// LoadResult contains the result of loading and processing flags.
type LoadResult struct {
	ConfigPath string
	Flags      *FlagsConfig
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
		Global: map[string]FlagInfo{
			"debug":            {Type: FlagTypeBool, DefaultValue: false, Description: "Enable debug output"},
			"disableAnalytics": {Type: FlagTypeBool, DefaultValue: false, Description: "Disable analytics"},
			"noTty":            {Type: FlagTypeBool, DefaultValue: false, Description: "Disable TTY"},
			"workdir":          {Type: FlagTypeString, DefaultValue: "", Description: "Working directory"},
			"outdir":           {Type: FlagTypeString, DefaultValue: "", Description: "Output directory"},
			"log":              {Type: FlagTypeString, DefaultValue: "", Description: "Log file path"},
			"gitProtocol":      {Type: FlagTypeString, DefaultValue: "https", Description: "Git protocol to use"},
		},
		Apply: map[string]FlagInfo{
			"phase": {Type: FlagTypeString, DefaultValue: "", Description: "Limit execution to specific phase"},
			"startFrom": {
				Type:         FlagTypeString,
				DefaultValue: "",
				Description:  "Start execution from specific phase",
			},
			"distroLocation":      {Type: FlagTypeString, DefaultValue: "", Description: "Distribution location"},
			"distroPatches":       {Type: FlagTypeString, DefaultValue: "", Description: "Distribution patches location"},
			"binPath":             {Type: FlagTypeString, DefaultValue: "", Description: "Binary path"},
			"skipNodesUpgrade":    {Type: FlagTypeBool, DefaultValue: false, Description: "Skip nodes upgrade"},
			"skipDepsDownload":    {Type: FlagTypeBool, DefaultValue: false, Description: "Skip dependencies download"},
			"skipDepsValidation":  {Type: FlagTypeBool, DefaultValue: false, Description: "Skip dependencies validation"},
			"dryRun":              {Type: FlagTypeBool, DefaultValue: false, Description: "Dry run mode"},
			"vpnAutoConnect":      {Type: FlagTypeBool, DefaultValue: false, Description: "Auto connect VPN"},
			"skipVpnConfirmation": {Type: FlagTypeBool, DefaultValue: false, Description: "Skip VPN confirmation"},
			"force":               {Type: FlagTypeStringSlice, DefaultValue: []string{}, Description: "Force options"},
			"postApplyPhases":     {Type: FlagTypeStringSlice, DefaultValue: []string{}, Description: "Post apply phases"},
			"timeout": {
				Type:         FlagTypeInt,
				DefaultValue: DefaultTimeoutSeconds,
				Description:  "Timeout in seconds",
			},
			"podRunningCheckTimeout": {
				Type:         FlagTypeInt,
				DefaultValue: DefaultPodRunningCheckTimeout,
				Description:  "Pod running check timeout",
			},
			"upgrade":             {Type: FlagTypeBool, DefaultValue: false, Description: "Enable upgrade mode"},
			"upgradePathLocation": {Type: FlagTypeString, DefaultValue: "", Description: "Upgrade path location"},
			"upgradeNode":         {Type: FlagTypeString, DefaultValue: "", Description: "Specific node to upgrade"},
		},
		Delete: map[string]FlagInfo{
			"phase":               {Type: FlagTypeString, DefaultValue: "", Description: "Limit execution to specific phase"},
			"startFrom":           {Type: FlagTypeString, DefaultValue: "", Description: "Start execution from specific phase"},
			"distroLocation":      {Type: FlagTypeString, DefaultValue: "", Description: "Distribution location"},
			"distroPatches":       {Type: FlagTypeString, DefaultValue: "", Description: "Distribution patches location"},
			"binPath":             {Type: FlagTypeString, DefaultValue: "", Description: "Binary path"},
			"dryRun":              {Type: FlagTypeBool, DefaultValue: false, Description: "Dry run mode"},
			"skipVpnConfirmation": {Type: FlagTypeBool, DefaultValue: false, Description: "Skip VPN confirmation"},
			"autoApprove":         {Type: FlagTypeBool, DefaultValue: false, Description: "Auto approve deletion"},
		},
		Create: map[string]FlagInfo{
			"name":     {Type: FlagTypeString, DefaultValue: "", Description: "Cluster name"},
			"version":  {Type: FlagTypeString, DefaultValue: "", Description: "Distribution version"},
			"provider": {Type: FlagTypeString, DefaultValue: "", Description: "Provider type"},
			"path":     {Type: FlagTypeString, DefaultValue: "pki", Description: "Path where to save PKI files"},
			"etcd":     {Type: FlagTypeBool, DefaultValue: false, Description: "Create PKI only for etcd"},
			"controlplane": {
				Type:         FlagTypeBool,
				DefaultValue: false,
				Description:  "Create PKI only for Kubernetes control plane",
			},
		},
		Get: map[string]FlagInfo{
			"binPath":            {Type: FlagTypeString, DefaultValue: "", Description: "Binary path"},
			"distroLocation":     {Type: FlagTypeString, DefaultValue: "", Description: "Distribution location"},
			"skipDepsDownload":   {Type: FlagTypeBool, DefaultValue: false, Description: "Skip dependencies download"},
			"skipDepsValidation": {Type: FlagTypeBool, DefaultValue: false, Description: "Skip dependencies validation"},
		},
		Diff: map[string]FlagInfo{
			"phase":               {Type: FlagTypeString, DefaultValue: "", Description: "Limit execution to specific phase"},
			"distroLocation":      {Type: FlagTypeString, DefaultValue: "", Description: "Distribution location"},
			"distroPatches":       {Type: FlagTypeString, DefaultValue: "", Description: "Distribution patches location"},
			"binPath":             {Type: FlagTypeString, DefaultValue: "", Description: "Binary path"},
			"upgradePathLocation": {Type: FlagTypeString, DefaultValue: "", Description: "Upgrade path location"},
		},
		Tools: map[string]FlagInfo{},
		Validate: map[string]FlagInfo{
			"distroLocation": {Type: FlagTypeString, DefaultValue: "", Description: "Distribution location"},
			"distroPatches":  {Type: FlagTypeString, DefaultValue: "", Description: "Distribution patches location"},
		},
		Download: map[string]FlagInfo{
			"binPath":        {Type: FlagTypeString, DefaultValue: "", Description: "Binary path"},
			"distroLocation": {Type: FlagTypeString, DefaultValue: "", Description: "Distribution location"},
			"distroPatches":  {Type: FlagTypeString, DefaultValue: "", Description: "Distribution patches location"},
		},
		Connect: map[string]FlagInfo{},
		Renew:   map[string]FlagInfo{},
		Dump:    map[string]FlagInfo{},
	}
}
