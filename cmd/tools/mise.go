// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tools

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/sighupio/furyctl/internal/analytics"
	"github.com/sighupio/furyctl/internal/app"
	iox "github.com/sighupio/furyctl/internal/x/io"
)

const (
	// FilePermissions represents the file permissions for mise.toml.
	FilePermissions = 0o600
)

// MiseConfig represents the structure of a mise.toml file.
type MiseConfig struct {
	Tools map[string]string `toml:"tools"`
	// Note: We use a map[string]any to preserve other sections
	// that might exist in the mise configuration.
	Other map[string]any `toml:",inline"`
}

// RevertOptions contains options for the revert operation.
type RevertOptions struct {
	SkipConfirmation bool
}

func NewMiseCmd() *cobra.Command {
	var cmdEvent analytics.Event

	miseCmd := &cobra.Command{
		Use:   "mise",
		Short: "Generate or update mise.toml with downloaded tool paths",
		Long: `Generate or update mise.toml configuration file with furyctl-downloaded tools.

This command creates or updates a mise.toml file in the current directory,
configuring mise to use the tools downloaded by furyctl instead of installing
its own versions.

If a mise.toml file already exists, only the tools section will be updated,
preserving all other configuration.

Examples:
  # Generate or update mise.toml
  furyctl tools mise

  # After running this command, mise will automatically use furyctl tools
  mise install    # No downloads needed, points to furyctl binaries
  kubectl version # Uses furyctl's kubectl version`,
		PreRun: func(cmd *cobra.Command, _ []string) {
			SetupToolsAnalytics(&cmdEvent, cmd)

			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				logrus.Fatalf("error while binding flags: %v", err)
			}
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctn := app.GetContainerInstance()
			tracker := ctn.Tracker()
			defer tracker.Flush()

			// Extract flags.
			flags := SharedFlags{
				BinPath:          viper.GetString("bin-path"),
				Config:           viper.GetString("config"),
				DistroLocation:   viper.GetString("distro-location"),
				SkipDepsDownload: viper.GetBool("skip-deps-download"),
				Debug:            viper.GetBool("debug"),
				GitProtocol:      viper.GetString("git-protocol"),
				OutDir:           viper.GetString("outdir"),
			}

			revert := viper.GetBool("revert")
			force := viper.GetBool("force")

			// Discover available tools.
			tools, err := DiscoverTools(flags)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return err
			}

			if len(tools) == 0 {
				cmd.Printf("No tools found in %s\n", flags.BinPath)
				if flags.SkipDepsDownload {
					cmd.Printf("Run 'furyctl download dependencies' first to download tools\n")
				} else {
					cmd.Printf("Unable to download dependencies automatically\n")
				}

				return nil
			}

			// Handle revert mode.
			if revert {
				if err := RevertMiseConfig(tools, RevertOptions{SkipConfirmation: force}, cmd); err != nil {
					cmdEvent.AddErrorMessage(err)
					tracker.Track(cmdEvent)

					return fmt.Errorf("failed to revert mise.toml: %w", err)
				}

				cmdEvent.AddSuccessMessage("Reverted furyctl tools from mise.toml")
				tracker.Track(cmdEvent)

				return nil
			}

			// Generate or update mise configuration.
			if err := updateMiseConfig(tools); err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("failed to update mise.toml: %w", err)
			}

			cmd.Printf("Updated mise.toml with %d tools:\n", len(tools))
			for _, tool := range tools {
				cmd.Printf("  %s = %s\n", tool.Name, tool.Version)
			}

			cmdEvent.AddSuccessMessage(fmt.Sprintf("Updated mise.toml with %d tools", len(tools)))
			tracker.Track(cmdEvent)

			return nil
		},
	}

	miseCmd.Flags().StringP(
		"bin-path",
		"b",
		"",
		"Path to the folder where all the dependencies' binaries are installed",
	)

	miseCmd.Flags().StringP(
		"config",
		"c",
		"furyctl.yaml",
		"Path to the configuration file",
	)

	miseCmd.Flags().StringP(
		"distro-location",
		"",
		"",
		"Location where to download schemas, defaults and the distribution manifests from. "+
			"It can either be a local path (eg: /path/to/distribution) or "+
			"a remote URL (eg: git::git@github.com:sighupio/distribution?depth=1&ref=BRANCH_NAME). "+
			"Any format supported by hashicorp/go-getter can be used",
	)

	miseCmd.Flags().Bool(
		"skip-deps-download",
		false,
		"Skip downloading the binaries",
	)

	miseCmd.Flags().Bool(
		"revert",
		false,
		"Remove furyctl-managed tools from mise.toml instead of adding them",
	)

	miseCmd.Flags().Bool(
		"force",
		false,
		"Skip confirmation prompt when reverting tools",
	)

	return miseCmd
}

// updateMiseConfig creates or updates the mise.toml file with furyctl tools.
func updateMiseConfig(tools []ToolInfo) error {
	const miseFile = "mise.toml"

	// Load existing configuration or create new one.
	config, err := loadMiseConfig(miseFile)
	if err != nil {
		return fmt.Errorf("failed to load existing mise config: %w", err)
	}

	// Update tools section with absolute paths.
	for _, tool := range tools {
		// Get absolute path to the tool's directory (not the binary itself).
		// Mise expects the directory to contain a binary with the same name as the tool.
		toolDir := filepath.Dir(tool.BinaryPath)
		absPath, err := filepath.Abs(toolDir)
		if err != nil { //nolint:wsl // gofumpt and wsl disagree on formatting
			return fmt.Errorf("failed to get absolute path for %s: %w", tool.Name, err)
		}

		// Use mise's path: syntax to specify exact tool location.
		config.Tools[tool.Name] = "path:" + absPath
	}

	// Write updated configuration back to file.
	if err := saveMiseConfig(miseFile, config); err != nil {
		return fmt.Errorf("failed to save mise config: %w", err)
	}

	return nil
}

// loadMiseConfig loads existing mise.toml file or returns empty config.
func loadMiseConfig(filename string) (*MiseConfig, error) {
	config := &MiseConfig{
		Tools: make(map[string]string),
		Other: make(map[string]any),
	}

	// Check if file exists.
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		// File doesn't exist, return empty config.
		return config, nil
	}

	// Read existing file.
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", filename, err)
	}

	// Parse existing TOML, preserving all sections.
	var rawConfig map[string]any
	if err := toml.Unmarshal(data, &rawConfig); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", filename, err)
	}

	// Extract tools section if it exists.
	if toolsSection, exists := rawConfig["tools"]; exists {
		if toolsMap, ok := toolsSection.(map[string]any); ok {
			for k, v := range toolsMap {
				if strVal, ok := v.(string); ok {
					config.Tools[k] = strVal
				}
			}
		}

		delete(rawConfig, "tools") // Remove from other sections.
	}

	// Store all other sections to preserve them.
	config.Other = rawConfig

	return config, nil
}

// saveMiseConfig saves the mise configuration to file.
func saveMiseConfig(filename string, config *MiseConfig) error {
	// Combine tools section with other sections.
	// This intermediate map is used to preserve the order of the sections in the final TOML file,
	// with the "tools" section appearing at the end, which is a common convention.
	combined := make(map[string]any)

	// Add other sections first.
	for k, v := range config.Other {
		combined[k] = v
	}

	// Add tools section.
	if len(config.Tools) > 0 {
		combined["tools"] = config.Tools
	}

	// Marshal to TOML.
	data, err := toml.Marshal(combined)
	if err != nil {
		return fmt.Errorf("failed to marshal TOML: %w", err)
	}

	// Write to file.
	if err := os.WriteFile(filename, data, FilePermissions); err != nil {
		return fmt.Errorf("failed to write %s: %w", filename, err)
	}

	return nil
}

// RevertMiseConfig removes furyctl-managed tools from mise.toml.
func RevertMiseConfig(tools []ToolInfo, opts RevertOptions, cmd *cobra.Command) error {
	const miseFile = "mise.toml"

	// Check if mise.toml exists.
	if _, err := os.Stat(miseFile); os.IsNotExist(err) {
		cmd.Printf("No %s file found, nothing to revert\n", miseFile)

		return nil
	}

	// Load existing configuration.
	config, err := loadMiseConfig(miseFile)
	if err != nil {
		return fmt.Errorf("failed to load existing mise config: %w", err)
	}

	// Identify furyctl-managed tools that exist in current config.
	toolsToRemove := IdentifyFuryctlTools(tools, config)
	if len(toolsToRemove) == 0 {
		cmd.Printf("No furyctl-managed tools found in %s, nothing to revert\n", miseFile)

		return nil
	}

	// Show what will be removed and get confirmation (unless force is used).
	if !opts.SkipConfirmation {
		confirmed, err := confirmRevert(toolsToRemove, cmd)
		if err != nil {
			return fmt.Errorf("error getting user confirmation: %w", err)
		}

		if !confirmed {
			cmd.Printf("Revert cancelled\n")

			return nil
		}
	}

	// Remove the tools from configuration.

	for _, toolName := range toolsToRemove {
		delete(config.Tools, toolName)
	}

	// Save updated configuration.
	if err := saveMiseConfig(miseFile, config); err != nil {
		return fmt.Errorf("failed to save mise config: %w", err)
	}

	cmd.Printf("Removed %d furyctl-managed tools from %s:\n", len(toolsToRemove), miseFile)

	for _, toolName := range toolsToRemove {
		cmd.Printf("  %s\n", toolName)
	}

	return nil
}

// IdentifyFuryctlTools identifies which discovered tools exist in the current mise config.
func IdentifyFuryctlTools(tools []ToolInfo, config *MiseConfig) []string {
	var toolsToRemove []string

	for _, tool := range tools {
		if _, exists := config.Tools[tool.Name]; exists {
			toolsToRemove = append(toolsToRemove, tool.Name)
		}
	}

	return toolsToRemove
}

// confirmRevert asks the user to confirm the revert operation.
func confirmRevert(toolsToRemove []string, cmd *cobra.Command) (bool, error) {
	cmd.Printf("\nThe following furyctl-managed tools will be removed from mise.toml:\n")

	for _, toolName := range toolsToRemove {
		cmd.Printf("  %s\n", toolName)
	}

	cmd.Printf("\nAre you sure you want to continue? Only 'yes' will be accepted to confirm.\n")

	prompter := iox.NewPrompter(bufio.NewReader(os.Stdin))
	confirmed, err := prompter.Ask("yes")
	if err != nil { //nolint:wsl // gofumpt and wsl disagree on formatting
		return false, fmt.Errorf("error reading user input: %w", err)
	}

	return confirmed, nil
}
