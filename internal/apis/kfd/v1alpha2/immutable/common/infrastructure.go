// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package common

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/furyctl/internal/cluster"
	iox "github.com/sighupio/furyctl/internal/x/io"
	"github.com/sighupio/furyctl/pkg/template"
)

const (
	// NetworkAddressParts is the expected number of parts in a network address.
	networkAddressParts = 2

	// Default values (used as fallbacks if not provided in config).
	defaultFlatcarVersion = "4206.0.0"
	defaultSSHUser        = "core"
	defaultInstallDisk    = "/dev/sda"
)

var (
	ErrInfraConfigNotFound    = errors.New("infrastructure config not found or invalid")
	ErrIPXEServerNotFound     = errors.New("ipxeServer config not found")
	ErrIPXEServerURLNotFound  = errors.New("ipxeServer.url not found")
	ErrSSHConfigNotFound      = errors.New("ssh config not found")
	ErrSSHKeyPathNotFound     = errors.New("ssh.keyPath not found")
	ErrNodesNotFound          = errors.New("infrastructure.nodes not found or invalid")
	ErrKubeConfigNotFound     = errors.New("kubernetes config not found")
	ErrControlPlaneNotFound   = errors.New("kubernetes.controlPlane not found")
	ErrControlMembersNotFound = errors.New("kubernetes.controlPlane.members not found")
	ErrStorageNotFound        = errors.New("storage config not found for node")
	ErrNetworkNotFound        = errors.New("network config not found for node")
	ErrNetworkEthersNotFound  = errors.New("network.ethernets not found for node")
	ErrButaneFatalErrors      = errors.New("butane translation has fatal errors")
)

// Infrastructure handles the infrastructure phase for Immutable kind.
type Infrastructure struct {
	*cluster.OperationPhase

	ConfigPath string
	ConfigData map[string]any
	DistroPath string

	// Configuration values (with defaults merged).
	flatcarVersion string
	sshUser        string
	installDisk    string
}

// nodeInfo represents processed node information.
type nodeInfo struct {
	Hostname       string
	MAC            string
	IP             string
	Gateway        string
	DNS            string
	Netmask        string
	Role           string
	InstallDisk    string
	SSHUser        string
	SSHKeys        []string
	IPXEServerURL  string
	FlatcarVersion string
}

// networkInfo represents network configuration for a node.
type networkInfo struct {
	IP      string
	Gateway string
	DNS     string
	Netmask string
}

// Prepare generates all infrastructure files.
func (i *Infrastructure) Prepare() error {
	// Initialize configuration values from merged config.
	i.initializeConfigValues()

	if err := i.CreateRootFolder(); err != nil {
		return fmt.Errorf("error creating infrastructure folder: %w", err)
	}

	if err := i.CreateFolderStructure(); err != nil {
		return fmt.Errorf("error creating folder structure: %w", err)
	}

	nodes, err := i.extractNodes()
	if err != nil {
		return fmt.Errorf("error extracting nodes: %w", err)
	}

	// Set flatcarVersion from Infrastructure config for all nodes.
	for idx := range nodes {
		nodes[idx].FlatcarVersion = i.flatcarVersion
	}

	logrus.Infof("Generating configurations for %d nodes...", len(nodes))

	// Render Butane templates from fury-distribution.
	if err := i.renderButaneTemplates(nodes); err != nil {
		return fmt.Errorf("error rendering butane templates: %w", err)
	}

	// Post-process: convert .bu to .ign for each node.
	for idx, node := range nodes {
		if err := i.generateNodeConfigs(idx, node); err != nil {
			return fmt.Errorf("error generating configs for node %s: %w", node.Hostname, err)
		}
	}

	logrus.Info("Node configurations generated successfully")

	return nil
}

// initializeConfigValues initializes configuration values from the merged config.
// Values are taken from the merged configuration (defaults + user overrides),
// with fallback to constants if not present.
func (i *Infrastructure) initializeConfigValues() {
	// Get infrastructure config section.
	infraConfig := i.getInfrastructureConfig()

	// Extract flatcarVersion.
	if version, ok := infraConfig["flatcarVersion"].(string); ok {
		i.flatcarVersion = version
	} else {
		i.flatcarVersion = defaultFlatcarVersion
	}

	// Extract sshUser.
	if sshConfig, ok := infraConfig["ssh"].(map[string]any); ok {
		if user, ok := sshConfig["username"].(string); ok {
			i.sshUser = user
		} else {
			i.sshUser = defaultSSHUser
		}
	} else {
		i.sshUser = defaultSSHUser
	}

	// Extract installDisk default.
	if disk, ok := infraConfig["installDisk"].(string); ok {
		i.installDisk = disk
	} else {
		i.installDisk = defaultInstallDisk
	}
}

// getInfrastructureConfig returns the infrastructure configuration section.
func (i *Infrastructure) getInfrastructureConfig() map[string]any {
	specConfig, ok := i.ConfigData["spec"].(map[string]any)
	if !ok {
		return make(map[string]any)
	}

	infraConfig, ok := specConfig["infrastructure"].(map[string]any)
	if !ok {
		return make(map[string]any)
	}

	return infraConfig
}

// renderButaneTemplates uses CopyFromTemplate to generate Butane files from fury-distribution.
func (i *Infrastructure) renderButaneTemplates(nodes []nodeInfo) error {
	// 1. Prepare configuration for templates.
	nodesData := make([]map[string]any, len(nodes))

	for idx, node := range nodes {
		nodesData[idx] = map[string]any{
			"ID":              idx,
			"Hostname":        node.Hostname,
			"MAC":             node.MAC,
			"IP":              node.IP,
			"Gateway":         node.Gateway,
			"DNS":             node.DNS,
			"Netmask":         node.Netmask,
			"Role":            node.Role,
			"InstallDisk":     node.InstallDisk,
			"SSHUser":         node.SSHUser,
			"SSHKeys":         node.SSHKeys,
			"IPXEServerURL":   node.IPXEServerURL,
			"FlatcarVersion":  i.flatcarVersion,
		}
	}

	// 2. Create config for templates.
	cfg := template.Config{
		Data: map[string]map[any]any{
			"data": {
				"nodes":          nodesData,
				"flatcarVersion": i.flatcarVersion,
				"ipxeServerURL":  nodesData[0]["IPXEServerURL"],
			},
		},
	}

	// 3. Determine source path of templates in fury-distribution.
	sourcePath := filepath.Join(
		i.DistroPath,
		"templates",
		"infrastructure",
		"immutable",
		"butane",
	)

	// 4. Use CopyFromTemplate to render templates.
	targetPath := filepath.Join(i.Path, "butane")

	if err := i.CopyFromTemplate(
		cfg,
		"immutable-infrastructure",
		sourcePath,
		targetPath,
		i.ConfigPath,
	); err != nil {
		return fmt.Errorf("error copying from templates: %w", err)
	}

	// 5. Post-process: Split multi-document YAML files by node.
	if err := i.splitButaneTemplates(); err != nil {
		return fmt.Errorf("error splitting butane templates: %w", err)
	}

	logrus.Info("Butane templates rendered from fury-distribution")

	return nil
}

// splitButaneTemplates splits the multi-document YAML files generated by CopyFromTemplate
// into individual files per node in the install/ directory.
func (i *Infrastructure) splitButaneTemplates() error {
	// Process each role's template file.
	roles := []string{"controlplane", "worker", "loadbalancer"}

	for _, role := range roles {
		templateFile := filepath.Join(i.Path, "butane", role+".bu")

		// Check if file exists.
		if _, err := os.Stat(templateFile); os.IsNotExist(err) {
			// Skip if role file doesn't exist (e.g., no load balancers in config).
			continue
		}

		// Read the multi-document YAML.
		content, err := os.ReadFile(templateFile)
		if err != nil {
			return fmt.Errorf("error reading %s: %w", templateFile, err)
		}

		// Split by "---\n" to get individual node documents.
		docs := splitYAMLDocuments(string(content))

		// Write each document to install/ directory.
		for _, doc := range docs {
			if doc == "" {
				continue
			}

			// Extract hostname from document.
			hostname := extractHostnameFromButane(doc)
			if hostname == "" {
				return fmt.Errorf("could not extract hostname from document in %s", templateFile)
			}

			// Write to install/ directory.
			installPath := filepath.Join(i.Path, "butane", "install", hostname+".bu")

			if err := os.WriteFile(installPath, []byte(doc), 0o644); err != nil {
				return fmt.Errorf("error writing %s: %w", installPath, err)
			}
		}

		// Remove the original multi-document file.
		if err := os.Remove(templateFile); err != nil {
			return fmt.Errorf("error removing %s: %w", templateFile, err)
		}
	}

	return nil
}

// CreateFolderStructure creates the directory structure declaratively.
func (i *Infrastructure) CreateFolderStructure() error {
	folders := []string{
		filepath.Join(i.Path, "butane", "install"),
		filepath.Join(i.Path, "butane", "bootstrap"),
		filepath.Join(i.Path, "ignition", "install"),
	}

	for _, folder := range folders {
		if err := os.MkdirAll(folder, iox.FullPermAccess); err != nil {
			return fmt.Errorf("error creating folder %s: %w", folder, err)
		}
	}

	return nil
}

// splitYAMLDocuments splits a multi-document YAML string by "---" separator.
func splitYAMLDocuments(content string) []string {
	// Split by "---" which is the YAML document separator.
	parts := []string{}

	// We need to handle different variations of the separator.
	currentDoc := ""

	for _, line := range splitLines(content) {
		// Check if this line is a document separator.
		if line == "---" {
			// Save current document if it has content.
			if len(currentDoc) > 0 {
				parts = append(parts, currentDoc)
			}

			currentDoc = ""

			continue
		}

		// Add line to current document.
		if len(currentDoc) > 0 {
			currentDoc += "\n"
		}

		currentDoc += line
	}

	// Add the last document.
	if len(currentDoc) > 0 {
		parts = append(parts, currentDoc)
	}

	return parts
}

// splitLines splits a string by newlines.
func splitLines(content string) []string {
	lines := []string{}
	currentLine := ""

	for _, ch := range content {
		if ch == '\n' {
			lines = append(lines, currentLine)
			currentLine = ""
		} else {
			currentLine += string(ch)
		}
	}

	// Add the last line if it's not empty.
	if len(currentLine) > 0 {
		lines = append(lines, currentLine)
	}

	return lines
}

// extractHostnameFromButane extracts the hostname from a Butane YAML document.
// It looks for the "inline:" value in the /etc/hostname file definition.
func extractHostnameFromButane(content string) string {
	lines := splitLines(content)
	foundHostnamePath := false

	for i, line := range lines {
		// Look for the hostname file path.
		if foundHostnamePath {
			// Next line after "contents:" should have "inline:" with the hostname.
			if len(line) > 0 && (line[0] == ' ' || line[0] == '\t') {
				// Check if this line contains "inline:".
				if idx := findSubstring(line, "inline:"); idx >= 0 {
					// Extract the hostname after "inline:".
					hostname := line[idx+7:] // Skip "inline:"

					// Trim spaces.
					hostname = trimSpaces(hostname)

					return hostname
				}
			}
		}

		// Look for "path: /etc/hostname".
		if idx := findSubstring(line, "path:"); idx >= 0 {
			if idx2 := findSubstring(line, "/etc/hostname"); idx2 >= 0 {
				// Found the hostname file definition, next "inline:" will have the hostname.
				foundHostnamePath = true

				// Also check if "contents:" is in the next few lines.
				for j := i + 1; j < i+5 && j < len(lines); j++ {
					if idx3 := findSubstring(lines[j], "inline:"); idx3 >= 0 {
						hostname := lines[j][idx3+7:]
						hostname = trimSpaces(hostname)

						return hostname
					}
				}
			}
		}
	}

	return ""
}

// findSubstring finds a substring in a string and returns its index, or -1 if not found.
func findSubstring(s, substr string) int {
	if len(substr) == 0 {
		return 0
	}

	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}

	return -1
}

// trimSpaces removes leading and trailing spaces from a string.
func trimSpaces(s string) string {
	start := 0
	end := len(s)

	// Find first non-space character.
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}

	// Find last non-space character.
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}

	return s[start:end]
}
