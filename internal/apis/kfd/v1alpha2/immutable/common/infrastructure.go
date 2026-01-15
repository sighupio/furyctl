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
	"github.com/sighupio/furyctl/internal/tool/butane"
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

// extractNodes processes the user configuration and extracts structured node information.
func (i *Infrastructure) extractNodes() ([]nodeInfo, error) {
	infraConfig := i.getInfrastructureConfig()

	// 1. Get kubernetes config for role determination.
	kubeConfig, err := i.getKubernetesConfig()
	if err != nil {
		return nil, fmt.Errorf("error getting kubernetes config: %w", err)
	}

	// 2. Extract SSH public keys.
	sshConfig, ok := infraConfig["ssh"].(map[string]any)
	if !ok {
		return nil, ErrSSHConfigNotFound
	}

	sshKeys, err := i.readSSHPublicKeys(sshConfig)
	if err != nil {
		return nil, fmt.Errorf("error reading SSH public keys: %w", err)
	}

	// 3. Extract iPXE server URL.
	ipxeConfig, ok := infraConfig["ipxeServer"].(map[string]any)
	if !ok {
		return nil, ErrIPXEServerNotFound
	}

	ipxeURL, ok := ipxeConfig["url"].(string)
	if !ok || ipxeURL == "" {
		return nil, ErrIPXEServerURLNotFound
	}

	// 4. Extract nodes list.
	nodesAny, ok := infraConfig["nodes"]
	if !ok {
		return nil, ErrNodesNotFound
	}

	nodesSlice, ok := nodesAny.([]any)
	if !ok {
		return nil, ErrNodesNotFound
	}

	if len(nodesSlice) == 0 {
		return nil, fmt.Errorf("%w: at least one node must be defined", ErrNodesNotFound)
	}

	// 5. Process each node.
	nodesList := make([]nodeInfo, 0, len(nodesSlice))

	for idx, nodeAny := range nodesSlice {
		nodeMap, ok := nodeAny.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("node at index %d is not a valid map", idx)
		}

		// Extract basic info.
		hostname, ok := nodeMap["hostname"].(string)
		if !ok || hostname == "" {
			return nil, fmt.Errorf("node at index %d: hostname is required", idx)
		}

		macAddress, ok := nodeMap["macAddress"].(string)
		if !ok || macAddress == "" {
			return nil, fmt.Errorf("node %s: macAddress is required", hostname)
		}

		// Determine role.
		role := i.determineNodeRole(hostname, kubeConfig)

		// Extract storage config.
		var installDisk string

		if storageAny, ok := nodeMap["storage"]; ok {
			if storageMap, ok := storageAny.(map[string]any); ok {
				if disk, ok := storageMap["installDisk"].(string); ok && disk != "" {
					installDisk = disk
				}
			}
		}

		// Use default if not specified.
		if installDisk == "" {
			installDisk = i.installDisk
		}

		// Extract network config.
		networkAny, ok := nodeMap["network"]
		if !ok {
			return nil, fmt.Errorf("%w for node %s", ErrNetworkNotFound, hostname)
		}

		netInfo, err := i.extractNetworkInfo(networkAny, hostname)
		if err != nil {
			return nil, fmt.Errorf("error extracting network info for node %s: %w", hostname, err)
		}

		// Build nodeInfo struct.
		nodesList = append(nodesList, nodeInfo{
			Hostname:       hostname,
			MAC:            macAddress,
			IP:             netInfo.IP,
			Gateway:        netInfo.Gateway,
			DNS:            netInfo.DNS,
			Netmask:        netInfo.Netmask,
			Role:           role,
			InstallDisk:    installDisk,
			SSHUser:        i.sshUser,
			SSHKeys:        sshKeys,
			IPXEServerURL:  ipxeURL,
			FlatcarVersion: i.flatcarVersion,
		})
	}

	logrus.Infof("Extracted %d nodes from configuration", len(nodesList))

	return nodesList, nil
}

// getKubernetesConfig returns the kubernetes configuration section.
func (i *Infrastructure) getKubernetesConfig() (map[string]any, error) {
	specConfig, ok := i.ConfigData["spec"].(map[string]any)
	if !ok {
		return nil, ErrKubeConfigNotFound
	}

	kubeConfig, ok := specConfig["kubernetes"].(map[string]any)
	if !ok {
		return nil, ErrKubeConfigNotFound
	}

	return kubeConfig, nil
}

// readSSHPublicKeys reads SSH public keys from the filesystem.
func (i *Infrastructure) readSSHPublicKeys(sshConfig map[string]any) ([]string, error) {
	keyPath, ok := sshConfig["keyPath"].(string)
	if !ok || keyPath == "" {
		return nil, ErrSSHKeyPathNotFound
	}

	// Expand environment variables (e.g., ${HOME}).
	keyPath = os.ExpandEnv(keyPath)

	// Read the public key file.
	keyContent, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("error reading SSH public key from %s: %w", keyPath, err)
	}

	// Trim whitespace and return as single-element slice.
	key := trimSpaces(string(keyContent))
	if key == "" {
		return nil, fmt.Errorf("SSH public key file %s is empty", keyPath)
	}

	return []string{key}, nil
}

// determineNodeRole determines the role of a node based on its hostname.
func (i *Infrastructure) determineNodeRole(hostname string, kubeConfig map[string]any) string {
	// 1. Check if in controlPlane.members[].
	if cpAny, ok := kubeConfig["controlPlane"]; ok {
		if cpMap, ok := cpAny.(map[string]any); ok {
			if membersAny, ok := cpMap["members"]; ok {
				if membersSlice, ok := membersAny.([]any); ok {
					for _, memberAny := range membersSlice {
						// Members are now objects with {hostname: "..."}
						if memberMap, ok := memberAny.(map[string]any); ok {
							if memberHostname, ok := memberMap["hostname"].(string); ok && memberHostname == hostname {
								return "controlplane"
							}
						}
						// Fallback for legacy string format (backward compatibility)
						if member, ok := memberAny.(string); ok && member == hostname {
							return "controlplane"
						}
					}
				}
			}
		}
	}

	// 2. Check if in nodeGroups[].nodes[].
	if groupsAny, ok := kubeConfig["nodeGroups"]; ok {
		if groupsSlice, ok := groupsAny.([]any); ok {
			for _, groupAny := range groupsSlice {
				if groupMap, ok := groupAny.(map[string]any); ok {
					if nodesAny, ok := groupMap["nodes"]; ok {
						if nodesSlice, ok := nodesAny.([]any); ok {
							for _, nodeAny := range nodesSlice {
								// Nodes are now objects with {hostname: "..."}
								if nodeMap, ok := nodeAny.(map[string]any); ok {
									if nodeHostname, ok := nodeMap["hostname"].(string); ok && nodeHostname == hostname {
										return "worker"
									}
								}
								// Fallback for legacy string format (backward compatibility)
								if node, ok := nodeAny.(string); ok && node == hostname {
									return "worker"
								}
							}
						}
					}
				}
			}
		}
	}

	// 3. Check if in loadBalancer config (future implementation).
	// TODO: Implement loadbalancer role detection when schema supports it.

	// 4. Default to worker if not found.
	logrus.Warnf("Node %s not found in controlPlane.members or nodeGroups.nodes, defaulting to worker role", hostname)

	return "worker"
}

// extractNetworkInfo extracts network configuration for a node.
func (i *Infrastructure) extractNetworkInfo(networkAny any, hostname string) (networkInfo, error) {
	networkMap, ok := networkAny.(map[string]any)
	if !ok {
		return networkInfo{}, fmt.Errorf("%w: network config is not a map", ErrNetworkNotFound)
	}

	// Extract ethernets configuration.
	ethernetsAny, ok := networkMap["ethernets"]
	if !ok {
		return networkInfo{}, fmt.Errorf("%w: ethernets not found", ErrNetworkEthersNotFound)
	}

	ethernetsMap, ok := ethernetsAny.(map[string]any)
	if !ok {
		return networkInfo{}, fmt.Errorf("%w: ethernets is not a map", ErrNetworkEthersNotFound)
	}

	// Get the first ethernet interface (usually eth0).
	// TODO: Support multiple interfaces and bonds in the future.
	var firstInterface map[string]any

	for _, ethAny := range ethernetsMap {
		if ethMap, ok := ethAny.(map[string]any); ok {
			firstInterface = ethMap
			break
		}
	}

	if firstInterface == nil {
		return networkInfo{}, fmt.Errorf("no valid ethernet interface found for node %s", hostname)
	}

	// Extract addresses (e.g., ["192.168.1.11/24"]).
	addressesAny, ok := firstInterface["addresses"]
	if !ok {
		return networkInfo{}, fmt.Errorf("addresses not found in ethernet config for node %s", hostname)
	}

	addressesSlice, ok := addressesAny.([]any)
	if !ok || len(addressesSlice) == 0 {
		return networkInfo{}, fmt.Errorf("addresses is empty or not a valid list for node %s", hostname)
	}

	// Get first address.
	addressWithCIDR, ok := addressesSlice[0].(string)
	if !ok {
		return networkInfo{}, fmt.Errorf("first address is not a string for node %s", hostname)
	}

	// Parse IP and netmask from CIDR notation (e.g., "192.168.1.11/24").
	ip, netmask, err := parseIPAndNetmask(addressWithCIDR)
	if err != nil {
		return networkInfo{}, fmt.Errorf("error parsing address %s for node %s: %w", addressWithCIDR, hostname, err)
	}

	// Extract gateway.
	gateway, ok := firstInterface["gateway"].(string)
	if !ok || gateway == "" {
		return networkInfo{}, fmt.Errorf("gateway not found in ethernet config for node %s", hostname)
	}

	// Extract nameservers.
	var dns string

	if nameserversAny, ok := firstInterface["nameservers"]; ok {
		if nameserversSlice, ok := nameserversAny.([]any); ok && len(nameserversSlice) > 0 {
			// Use first nameserver as primary DNS.
			if dnsStr, ok := nameserversSlice[0].(string); ok {
				dns = dnsStr
			}
		}
	}

	if dns == "" {
		// Use gateway as DNS if not specified (common in home labs).
		logrus.Warnf("No nameservers specified for node %s, using gateway %s as DNS", hostname, gateway)
		dns = gateway
	}

	return networkInfo{
		IP:      ip,
		Gateway: gateway,
		DNS:     dns,
		Netmask: netmask,
	}, nil
}

// parseIPAndNetmask parses an IP address with CIDR notation (e.g., "192.168.1.11/24")
// and returns the IP and netmask as separate strings.
func parseIPAndNetmask(cidr string) (ip, netmask string, err error) {
	// Split by '/'.
	parts := []string{}
	current := ""

	for _, ch := range cidr {
		if ch == '/' {
			if len(current) > 0 {
				parts = append(parts, current)
			}

			current = ""
		} else {
			current += string(ch)
		}
	}

	// Add last part.
	if len(current) > 0 {
		parts = append(parts, current)
	}

	if len(parts) != networkAddressParts {
		return "", "", fmt.Errorf("invalid CIDR format: %s (expected format: IP/CIDR)", cidr)
	}

	ip = parts[0]
	cidrBits := parts[1]

	// Validate IP format (simple check).
	if !isValidIP(ip) {
		return "", "", fmt.Errorf("invalid IP address: %s", ip)
	}

	return ip, cidrBits, nil
}

// isValidIP performs a simple validation of an IPv4 address.
func isValidIP(ip string) bool {
	// Split by '.'.
	octets := []string{}
	current := ""

	for _, ch := range ip {
		if ch == '.' {
			if len(current) > 0 {
				octets = append(octets, current)
			}

			current = ""
		} else {
			current += string(ch)
		}
	}

	// Add last octet.
	if len(current) > 0 {
		octets = append(octets, current)
	}

	// IPv4 must have 4 octets.
	if len(octets) != 4 {
		return false
	}

	// Each octet must be a number between 0-255.
	for _, octet := range octets {
		// Simple check: all characters must be digits.
		if len(octet) == 0 || len(octet) > 3 {
			return false
		}

		for _, ch := range octet {
			if ch < '0' || ch > '9' {
				return false
			}
		}
	}

	return true
}

// generateNodeConfigs converts Butane YAML to Ignition JSON for a node.
func (i *Infrastructure) generateNodeConfigs(idx int, node nodeInfo) error {
	// 1. Construct file paths.
	butanePath := filepath.Join(i.Path, "butane", "install", node.Hostname+".bu")
	ignitionPath := filepath.Join(i.Path, "ignition", "install", node.Hostname+".json")

	logrus.Debugf("Converting Butane to Ignition for node %s (%d/%d)", node.Hostname, idx+1, idx+1)

	// 2. Read Butane file.
	butaneContent, err := os.ReadFile(butanePath)
	if err != nil {
		return fmt.Errorf("error reading butane file %s: %w", butanePath, err)
	}

	// 3. Create Butane runner.
	runner := butane.NewRunner()
	runner.SetPretty(true)

	// 4. Convert Butane YAML to Ignition JSON.
	ignitionJSON, report, err := runner.ConvertWithReport(butaneContent)
	if err != nil {
		return fmt.Errorf("error converting butane to ignition for %s: %w", node.Hostname, err)
	}

	// 5. Check for fatal errors in report.
	if report.IsFatal() {
		return fmt.Errorf("%w for node %s: %s", ErrButaneFatalErrors, node.Hostname, report.String())
	}

	// 6. Log warnings if present.
	if len(report.Entries) > 0 {
		logrus.Warnf("Butane conversion warnings for %s: %s", node.Hostname, report.String())
	}

	// 7. Write Ignition JSON.
	if err := os.WriteFile(ignitionPath, ignitionJSON, 0o644); err != nil {
		return fmt.Errorf("error writing ignition file %s: %w", ignitionPath, err)
	}

	logrus.Infof("✓ Generated Ignition config for node %s", node.Hostname)

	return nil
}
