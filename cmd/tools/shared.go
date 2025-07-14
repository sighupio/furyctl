// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tools

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	distroconfig "github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/furyctl/internal/analytics"
	"github.com/sighupio/furyctl/internal/app"
	"github.com/sighupio/furyctl/internal/config"
	"github.com/sighupio/furyctl/internal/git"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	"github.com/sighupio/furyctl/pkg/dependencies"
	dist "github.com/sighupio/furyctl/pkg/distribution"
	netx "github.com/sighupio/furyctl/pkg/x/net"
)

const (
	versionPrefixLength = 3
	// AliasFormat generates bash aliases.
	AliasFormat OutputFormat = iota
	// FunctionFormat generates bash functions.
	FunctionFormat
)

var (
	// ErrConfigNotFound is returned when the configuration file is not found.
	ErrConfigNotFound = errors.New("configuration file not found")
	// ErrNoToolsFound is returned when no tools are found in the bin directory.
	ErrNoToolsFound = errors.New("no tools found in bin directory. Run 'furyctl download dependencies' first to download tools")
)

// SharedFlags contains common flags used by all tools commands.
type SharedFlags struct {
	BinPath          string
	Config           string
	DistroLocation   string
	SkipDepsDownload bool
	Debug            bool
	GitProtocol      string
	OutDir           string
}

// ToolInfo represents a discovered tool with its version and path.
type ToolInfo struct {
	Name       string
	Version    string
	BinaryPath string
	BinaryName string
}

// OutputFormat represents different shell integration formats.
type OutputFormat int

// DiscoverTools discovers all available tools and returns their information.
func DiscoverTools(flags SharedFlags) ([]ToolInfo, error) {
	// Validate configuration file exists.
	if _, err := os.Stat(flags.Config); os.IsNotExist(err) {
		return nil, fmt.Errorf("%w: %s", ErrConfigNotFound, flags.Config)
	}

	// Initialize paths and clients.
	outDir := flags.OutDir
	furyctlPath := flags.Config
	binPath := flags.BinPath

	if binPath == "" {
		binPath = path.Join(outDir, ".furyctl", "bin")
	}

	var err error
	binPath, err = filepath.Abs(binPath)
	if err != nil { //nolint:wsl // gofumpt and wsl disagree on formatting
		return nil, fmt.Errorf("error while getting absolute path for bin folder: %w", err)
	}

	// Default to https if no protocol specified or invalid.
	gitProtocol := flags.GitProtocol
	if gitProtocol == "" {
		gitProtocol = "https"
	}

	typedGitProtocol, err := git.NewProtocol(gitProtocol)
	if err != nil {
		// Fallback to https on error.
		typedGitProtocol = git.ProtocolHTTPS

		logrus.Debugf("Invalid git protocol %s, defaulting to https", gitProtocol)
	}

	// Init packages.
	execx.Debug = flags.Debug
	executor := execx.NewStdExecutor()
	client := netx.NewGoGetterClient()

	// Check if dependencies exist, if not try to download them.
	needsDownload := false
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		needsDownload = true
	} else {
		// Check if directory is empty.
		entries, err := os.ReadDir(binPath)
		if err == nil && len(entries) == 0 {
			needsDownload = true
		}
	}

	var distroManifest distroconfig.KFD

	if needsDownload {
		if flags.SkipDepsDownload {
			return nil, ErrNoToolsFound
		}

		// Download distribution and dependencies.
		var distrodl *dist.Downloader
		if flags.DistroLocation == "" {
			distrodl = dist.NewCachingDownloader(client, outDir, typedGitProtocol, "")
		} else {
			distrodl = dist.NewDownloader(client, typedGitProtocol, "")
		}

		// Validate base requirements.
		depsvl := dependencies.NewValidator(executor, binPath, furyctlPath, false)
		if err := depsvl.ValidateBaseReqs(); err != nil {
			return nil, fmt.Errorf("error while validating requirements: %w", err)
		}

		// Download the distribution.
		res, err := distrodl.Download(flags.DistroLocation, furyctlPath)

		if err != nil {
			return nil, fmt.Errorf("error while downloading distribution: %w", err)
		}

		basePath := path.Join(outDir, ".furyctl", res.MinimalConf.Metadata.Name)

		// Validate the furyctl.yaml file.
		if err := config.Validate(furyctlPath, res.RepoPath); err != nil {
			return nil, fmt.Errorf("error while validating configuration file: %w", err)
		}

		// Download the dependencies.
		depsdl := dependencies.NewCachingDownloader(client, outDir, basePath, binPath, typedGitProtocol)

		if _, err := depsdl.DownloadTools(res.DistroManifest); err != nil {
			return nil, fmt.Errorf("error while downloading tools: %w", err)
		}

		distroManifest = res.DistroManifest
	} else {
		// Load the distribution to get tool versions from kfd.yaml.
		var distrodl *dist.Downloader
		if flags.DistroLocation == "" {
			distrodl = dist.NewCachingDownloader(client, outDir, typedGitProtocol, "")
		} else {
			distrodl = dist.NewDownloader(client, typedGitProtocol, "")
		}

		res, err := distrodl.Download(flags.DistroLocation, furyctlPath)

		if err != nil {
			return nil, fmt.Errorf("error while downloading distribution: %w", err)
		}

		distroManifest = res.DistroManifest
	}

	// Generate tool information using kfd.yaml for correct versions.
	return generateToolsWithKFD(binPath, distroManifest), nil
}

// generateToolsWithKFD generates tool information using KFD manifest for correct tool versions.
func generateToolsWithKFD(binPath string, kfdManifest distroconfig.KFD) []ToolInfo {
	var tools []ToolInfo

	// Tool name mapping for special cases.
	toolBinaryMap := map[string]string{
		"awscli": "aws", // Awscli directory contains 'aws' binary.
	}

	// Extract tools from KFD manifest.
	toolsMap := make(map[string]string)

	// Add common tools.
	addCommonToolsToMap(toolsMap, kfdManifest.Tools.Common)

	// Add EKS-specific tools.
	addEKSToolsToMap(toolsMap, kfdManifest.Tools.Eks)

	// Generate tool information based on KFD manifest versions.
	for toolName, version := range toolsMap {
		// Clean version (remove >= prefix if present).
		cleanVersion := cleanVersionConstraint(version)

		// Determine binary name (handle special cases).
		binaryName := toolName
		if mappedName, exists := toolBinaryMap[toolName]; exists {
			binaryName = mappedName
		}

		// Construct full binary path.
		binaryPath := filepath.Join(binPath, toolName, cleanVersion, binaryName)

		// Check if binary exists.
		if _, err := os.Stat(binaryPath); err == nil {
			tools = append(tools, ToolInfo{
				Name:       toolName,
				Version:    cleanVersion,
				BinaryPath: binaryPath,
				BinaryName: binaryName,
			})
		} else {
			logrus.Debugf("Tool %s version %s not found at %s", toolName, cleanVersion, binaryPath)
		}
	}

	return tools
}

// addCommonToolsToMap extracts common tool versions from KFD config.
func addCommonToolsToMap(toolsMap map[string]string, commonTools distroconfig.KFDToolsCommon) {
	if commonTools.Furyagent.Version != "" {
		toolsMap["furyagent"] = commonTools.Furyagent.Version
	}

	if commonTools.Kubectl.Version != "" {
		toolsMap["kubectl"] = commonTools.Kubectl.Version
	}

	if commonTools.Kustomize.Version != "" {
		toolsMap["kustomize"] = commonTools.Kustomize.Version
	}

	if commonTools.Terraform.Version != "" {
		toolsMap["terraform"] = commonTools.Terraform.Version
	}

	if commonTools.Yq.Version != "" {
		toolsMap["yq"] = commonTools.Yq.Version
	}

	if commonTools.Kapp.Version != "" {
		toolsMap["kapp"] = commonTools.Kapp.Version
	}

	if commonTools.Helm.Version != "" {
		toolsMap["helm"] = commonTools.Helm.Version
	}

	if commonTools.Helmfile.Version != "" {
		toolsMap["helmfile"] = commonTools.Helmfile.Version
	}
}

// addEKSToolsToMap extracts EKS-specific tool versions from KFD config.
func addEKSToolsToMap(toolsMap map[string]string, eksTools distroconfig.KFDToolsEks) {
	if eksTools.Awscli.Version != "" {
		toolsMap["awscli"] = eksTools.Awscli.Version
	}
}

// cleanVersionConstraint removes version constraint prefixes like ">= ".
func cleanVersionConstraint(version string) string {
	// Remove ">= " prefix if present.
	if len(version) > versionPrefixLength && version[:versionPrefixLength] == ">= " {
		return version[versionPrefixLength:]
	}

	return version
}

// SetupToolsAnalytics initializes analytics for tools commands.
func SetupToolsAnalytics(cmdEvent *analytics.Event, cmd *cobra.Command) {
	*cmdEvent = analytics.NewCommandEvent(cobrax.GetFullname(cmd))
}

// CreateShellIntegrationCommand creates a command for shell integration with common logic.
func CreateShellIntegrationCommand(
	use, short, long string,
	format OutputFormat,
) *cobra.Command {
	var cmdEvent analytics.Event

	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Long:  long,
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

			// Discover available tools.
			tools, err := DiscoverTools(flags)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return err
			}

			// Generate and output shell integration.
			if len(tools) == 0 {
				cmd.Printf("# No tools found in %s\n", flags.BinPath)
				if flags.SkipDepsDownload {
					cmd.Printf("# Run 'furyctl download dependencies' first to download tools\n")
				} else {
					cmd.Printf("# Unable to download dependencies automatically\n")
				}
			} else {
				for _, tool := range tools {
					switch format {
					case AliasFormat:
						cmd.Printf("alias %s=\"%s\"\n", tool.Name, tool.BinaryPath)
					case FunctionFormat:
						cmd.Printf("%s() { \"%s\" \"$@\"; }\n", tool.Name, tool.BinaryPath)
					}
				}
			}

			var formatName string
			switch format {
			case AliasFormat:
				formatName = "aliases"
			case FunctionFormat:
				formatName = "functions"
			}

			cmdEvent.AddSuccessMessage(fmt.Sprintf("Generated %d %s", len(tools), formatName))
			tracker.Track(cmdEvent)

			return nil
		},
	}

	cmd.Flags().StringP(
		"bin-path",
		"b",
		"",
		"Path to the folder where all the dependencies' binaries are installed",
	)

	cmd.Flags().StringP(
		"config",
		"c",
		"furyctl.yaml",
		"Path to the configuration file",
	)

	cmd.Flags().StringP(
		"distro-location",
		"",
		"",
		"Location where to download schemas, defaults and the distribution manifests from. "+
			"It can either be a local path (eg: /path/to/distribution) or "+
			"a remote URL (eg: git::git@github.com:sighupio/distribution?depth=1&ref=BRANCH_NAME). "+
			"Any format supported by hashicorp/go-getter can be used",
	)

	cmd.Flags().Bool(
		"skip-deps-download",
		false,
		"Skip downloading the binaries",
	)

	return cmd
}
