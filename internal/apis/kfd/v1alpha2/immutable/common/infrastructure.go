// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package common

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	texttemplate "text/template"

	"github.com/hashicorp/go-getter"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"

	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/tool/butane"
	iox "github.com/sighupio/furyctl/internal/x/io"
	"github.com/sighupio/furyctl/pkg/template"
	netx "github.com/sighupio/furyctl/pkg/x/net"
)

const (
	// NetworkAddressParts is the expected number of parts in a network address.
	networkAddressParts = 2

	// File permission modes.
	filePermissionUserReadWrite = 0o600 // User read/write only.

	// Butane hostname extraction constants.
	inlineKeywordLength = 7 // Length of "inline:" keyword.
	maxLineLookAhead    = 5 // Maximum lines to look ahead for hostname.

	// Node role names.
	roleControlPlane = "controlplane"
	roleETCD         = "etcd"
	roleLoadBalancer = "loadbalancer"
	roleWorker       = "worker"

	// Default values (used as fallbacks if not provided in config).
	defaultOSVersion   = "4206.0.0" // Default Flatcar Container Linux version.
	defaultSSHUser     = "core"     // Default SSH user for Flatcar Container Linux.
	defaultInstallDisk = "/dev/sda" // Default installation disk.
)

var (
	ErrIPXEServerNotFound        = errors.New("ipxeServer config not found")
	ErrIPXEServerURLNotFound     = errors.New("ipxeServer.url not found")
	ErrSSHConfigNotFound         = errors.New("ssh config not found")
	ErrSSHKeyPathNotFound        = errors.New("ssh.keyPath not found")
	ErrNodesNotFound             = errors.New("infrastructure.nodes not found or invalid")
	ErrKubeConfigNotFound        = errors.New("kubernetes config not found")
	ErrControlPlaneNotFound      = errors.New("kubernetes.controlPlane not found")
	ErrControlMembersNotFound    = errors.New("kubernetes.controlPlane.members not found")
	ErrNetworkNotFound           = errors.New("network config not found for node")
	ErrNetworkEthersNotFound     = errors.New("network.ethernets not found for node")
	ErrButaneFatalErrors         = errors.New("butane translation has fatal errors")
	ErrNoKubernetesVersions      = errors.New("no Kubernetes versions defined in immutable manifest")
	ErrKubernetesVersionNotFound = errors.New("kubernetes version not found in immutable manifest")
	ErrFlatcarArtifactsNotFound  = errors.New("flatcar artifacts not found for architecture")
	ErrHostnameNotExtracted      = errors.New("could not extract hostname from document")
	ErrButaneConversionFatal     = errors.New("butane conversion fatal errors")
	ErrInvalidNodeMap            = errors.New("node is not a valid map")
	ErrNodeHostnameRequired      = errors.New("node hostname is required")
	ErrNodeMACRequired           = errors.New("node macAddress is required")
	ErrSSHKeyPathMustBeSpecified = errors.New("either ssh.privateKeyPath or ssh.keyPath (deprecated) must be specified")
	ErrSSHPublicKeyEmpty         = errors.New("SSH public key file is empty")
	ErrNoEthernetInterfaceFound  = errors.New("no valid ethernet interface found for node")
	ErrAddressesNotFound         = errors.New("addresses not found in ethernet config for node")
	ErrAddressesInvalid          = errors.New("addresses is empty or not a valid list for node")
	ErrAddressNotString          = errors.New("first address is not a string for node")
	ErrInvalidCIDRFormat         = errors.New("invalid CIDR format")
	ErrInvalidIPAddress          = errors.New("invalid IP address")
	ErrGatewayNotFound           = errors.New("gateway not found in ethernet config for node")
	ErrContainerdSysextNotFound  = errors.New("containerd sysext not found for kubernetes")
	ErrKubernetesSysextNotFound  = errors.New("kubernetes sysext not found for kubernetes")
	ErrChecksumMismatch          = errors.New("checksum mismatch")
)

// SysextVersions holds version information for systemd-sysext packages.
type SysextVersions struct {
	ContainerdVersion string
	RuncVersion       string
	CNIPluginsVersion string
	KubernetesVersion string
	KubernetesMajor   string
	KeepaliveVersion  string
	EtcdVersion       string
}

// immutableManifest represents the structure of immutable.yaml file.
// This file contains all versioning information for Kubernetes, sysext packages,
// and Flatcar Container Linux. It should be located in the installer repository
// (fury-kubernetes-immutable-installer) when ready.
type immutableManifest struct {
	//nolint:tagliatelle // YAML structure matches file.
	DefaultKubernetesVersion string                       `yaml:"default_kubernetes_version"`
	Kubernetes               map[string]kubernetesRelease `yaml:"kubernetes"`
}

// kubernetesRelease represents a Kubernetes version entry in immutable.yaml.
type kubernetesRelease struct {
	Sysext  []sysextPackage `yaml:"sysext"` // Array of sysext packages.
	Flatcar flatcarRelease  `yaml:"flatcar"`
}

// sysextPackage represents a systemd-sysext package configuration.
// Filename convention: {name}-{version}-{arch}.raw.
type sysextPackage struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
	//nolint:tagliatelle // YAML structure matches file.
	VersionMajorMinor string                    `yaml:"version_major_minor"`
	Arch              map[string]sysextArchInfo `yaml:"arch"` // Map of arch -> url + sha256.
}

// sysextArchInfo contains architecture-specific information.
type sysextArchInfo struct {
	URL    string `yaml:"url"`
	SHA256 string `yaml:"sha256,omitempty"` // Optional SHA256 for verification.
}

// flatcarRelease represents a Flatcar Container Linux version.
type flatcarRelease struct {
	Version string                 `yaml:"version"`
	Channel string                 `yaml:"channel"`
	Arch    map[string]flatcarArch `yaml:"arch"` // Map of arch -> boot artifacts.
}

// flatcarArch contains architecture-specific boot artifacts.
type flatcarArch struct {
	Kernel flatcarArtifact `yaml:"kernel"`
	Initrd flatcarArtifact `yaml:"initrd"`
	Image  flatcarArtifact `yaml:"image"`
}

// flatcarArtifact represents a single boot artifact (kernel, initrd, image).
type flatcarArtifact struct {
	Filename string `yaml:"filename"`
	URL      string `yaml:"url"`
	SHA256   string `yaml:"sha256,omitempty"` // Optional SHA256 for verification.
}

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
	sysextVersions SysextVersions
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
	Arch           string // CPU architecture: x86-64 or arm64.
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

	// Load sysext versions from installer defaults.
	if err := i.loadSysextVersions(); err != nil {
		return fmt.Errorf("error loading sysext versions: %w", err)
	}

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

	// Download assets for architectures used in the cluster.
	kubeVersion := i.getKubernetesVersion()
	manifestPath := filepath.Join(i.DistroPath, "installers", "immutable", "immutable.yaml")

	manifest, err := i.parseImmutableManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("error loading immutable manifest: %w", err)
	}

	release, ok := manifest.Kubernetes[kubeVersion]
	if !ok {
		return fmt.Errorf("%w: %s", ErrKubernetesVersionNotFound, kubeVersion)
	}

	usedArchitectures := i.extractUsedArchitectures(nodes)
	if err := i.downloadAssets(release, usedArchitectures); err != nil {
		return fmt.Errorf("error downloading assets: %w", err)
	}

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
		i.flatcarVersion = defaultOSVersion
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
	// 1. Load the full immutable manifest to pass all sysext info to templates.
	kubeVersion := i.getKubernetesVersion()
	manifestPath := filepath.Join(i.DistroPath, "installers", "immutable", "immutable.yaml")

	manifest, err := i.parseImmutableManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("error loading immutable manifest for templates: %w", err)
	}

	release, ok := manifest.Kubernetes[kubeVersion]
	if !ok {
		return fmt.Errorf("%w: %s", ErrKubernetesVersionNotFound, kubeVersion)
	}

	// 2. Prepare configuration for templates.
	nodesData := make([]map[string]any, len(nodes))

	for idx, node := range nodes {
		// Get Flatcar boot artifacts for this node's architecture.
		archInfo, ok := release.Flatcar.Arch[node.Arch]
		if !ok {
			return fmt.Errorf("%w: %s", ErrFlatcarArtifactsNotFound, node.Arch)
		}

		flatcarKernelURL := archInfo.Kernel.URL
		flatcarInitrdURL := archInfo.Initrd.URL
		flatcarImageURL := archInfo.Image.URL

		nodesData[idx] = map[string]any{
			"ID":               idx,
			"Hostname":         node.Hostname,
			"MAC":              node.MAC,
			"IP":               node.IP,
			"Gateway":          node.Gateway,
			"DNS":              node.DNS,
			"Netmask":          node.Netmask,
			"Role":             node.Role,
			"InstallDisk":      node.InstallDisk,
			"SSHUser":          node.SSHUser,
			"SSHKeys":          node.SSHKeys,
			"IPXEServerURL":    node.IPXEServerURL,
			"FlatcarVersion":   i.flatcarVersion,
			"Arch":             node.Arch,
			"FlatcarKernelURL": flatcarKernelURL,
			"FlatcarInitrdURL": flatcarInitrdURL,
			"FlatcarImageURL":  flatcarImageURL,
		}
	}

	// 3. Convert sysext packages to template-friendly format.
	sysextData := make(map[any]any)

	for _, pkg := range release.Sysext {
		pkgData := map[any]any{
			"name":    pkg.Name,
			"version": pkg.Version,
			"arch":    make(map[any]any),
		}

		if pkg.VersionMajorMinor != "" {
			pkgData["version_major_minor"] = pkg.VersionMajorMinor
		}

		// Add arch-specific info.
		for arch, archInfo := range pkg.Arch {
			archData := map[any]any{
				"url": archInfo.URL,
			}
			if archInfo.SHA256 != "" {
				archData["sha256"] = archInfo.SHA256
			}

			if archMap, ok := pkgData["arch"].(map[any]any); ok {
				archMap[arch] = archData
			}
		}

		sysextData[pkg.Name] = pkgData
	}

	// 4. Convert Flatcar boot artifacts to template-friendly format.
	flatcarData := map[any]any{
		"version": release.Flatcar.Version,
		"channel": release.Flatcar.Channel,
		"arch":    make(map[any]any),
	}

	for arch, archInfo := range release.Flatcar.Arch {
		artifactsData := map[any]any{
			"kernel": map[any]any{
				"filename": archInfo.Kernel.Filename,
				"url":      archInfo.Kernel.URL,
			},
			"initrd": map[any]any{
				"filename": archInfo.Initrd.Filename,
				"url":      archInfo.Initrd.URL,
			},
			"image": map[any]any{
				"filename": archInfo.Image.Filename,
				"url":      archInfo.Image.URL,
			},
		}

		// Add SHA256 if present.
		if archInfo.Kernel.SHA256 != "" {
			if kernelMap, ok := artifactsData["kernel"].(map[any]any); ok {
				kernelMap["sha256"] = archInfo.Kernel.SHA256
			}
		}

		if archInfo.Initrd.SHA256 != "" {
			if initrdMap, ok := artifactsData["initrd"].(map[any]any); ok {
				initrdMap["sha256"] = archInfo.Initrd.SHA256
			}
		}

		if archInfo.Image.SHA256 != "" {
			if imageMap, ok := artifactsData["image"].(map[any]any); ok {
				imageMap["sha256"] = archInfo.Image.SHA256
			}
		}

		if flatcarArchMap, ok := flatcarData["arch"].(map[any]any); ok {
			flatcarArchMap[arch] = artifactsData
		}
	}

	// 5. Create config for templates.
	cfg := template.Config{
		Data: map[string]map[any]any{
			"data": {
				"nodes":          nodesData,
				"flatcarVersion": i.flatcarVersion,
				"ipxeServerURL":  nodesData[0]["IPXEServerURL"],
				"sysext":         sysextData,
				"flatcar":        flatcarData,
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
	targetPath := filepath.Join(i.Path, "templates", "butane")

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

	// 6. Generate bootstrap templates that embed install ignition.
	if err := i.generateBootstrapTemplates(nodes); err != nil {
		return fmt.Errorf("error generating bootstrap templates: %w", err)
	}

	logrus.Info("Butane templates rendered from fury-distribution")

	return nil
}

// splitButaneTemplates splits the multi-document YAML files generated by CopyFromTemplate
// into individual files per node in the install/ directory.
func (i *Infrastructure) splitButaneTemplates() error {
	// Process each role's template file.
	roles := []string{"controlplane", "worker", "loadbalancer", "etcd"}

	for _, role := range roles {
		templateFile := filepath.Join(i.Path, "templates", "butane", role+".bu")

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
				return fmt.Errorf("%w in %s", ErrHostnameNotExtracted, templateFile)
			}

			// Write to templates/butane/install/ directory.
			installPath := filepath.Join(i.Path, "templates", "butane", "install", hostname+".bu")

			if err := os.WriteFile(installPath, []byte(doc), filePermissionUserReadWrite); err != nil {
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

// generateBootstrapTemplates generates bootstrap butane templates from install templates.
// Bootstrap templates embed the install ignition (compressed and base64 encoded) and handle
// the initial PXE boot -> disk installation workflow.
func (i *Infrastructure) generateBootstrapTemplates(nodes []nodeInfo) error {
	installDir := filepath.Join(i.Path, "templates", "butane", "install")
	bootstrapDir := filepath.Join(i.Path, "templates", "butane", "bootstrap")

	// Read all install butane files.
	entries, err := os.ReadDir(installDir)
	if err != nil {
		return fmt.Errorf("error reading install directory: %w", err)
	}

	// Get path to bootstrap template in fury-distribution.
	bootstrapTemplatePath := filepath.Join(
		i.DistroPath,
		"templates",
		"infrastructure",
		"immutable",
		"butane",
		"bootstrap.bu.tmpl",
	)

	// Check if template exists.
	if _, err := os.Stat(bootstrapTemplatePath); err != nil {
		return fmt.Errorf("bootstrap template not found at %s: %w", bootstrapTemplatePath, err)
	}

	// Read bootstrap template once.
	bootstrapTemplateContent, err := os.ReadFile(bootstrapTemplatePath)
	if err != nil {
		return fmt.Errorf("error reading bootstrap template: %w", err)
	}

	// Create butane runner.
	runner := butane.NewRunner()
	runner.SetPretty(true)

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".bu") {
			continue
		}

		installPath := filepath.Join(installDir, entry.Name())
		hostname := strings.TrimSuffix(entry.Name(), ".bu")

		logrus.Debugf("Generating bootstrap for %s", hostname)

		// 1. Read install butane file.
		butaneContent, err := os.ReadFile(installPath)
		if err != nil {
			return fmt.Errorf("error reading install butane %s: %w", installPath, err)
		}

		// 2. Convert install butane to ignition JSON.
		ignitionJSON, report, err := runner.ConvertWithReport(butaneContent)
		if err != nil {
			return fmt.Errorf("error converting %s to ignition: %w", installPath, err)
		}

		// Check for fatal errors in report.
		if report.IsFatal() {
			return fmt.Errorf("%w for %s: %s", ErrButaneConversionFatal, hostname, report.String())
		}

		// Log warnings if present.
		if len(report.Entries) > 0 {
			logrus.Warnf("Butane conversion warnings for %s: %s", hostname, report.String())
		}

		// 3. Compress with gzip.
		var gzipBuf bytes.Buffer
		gzipWriter := gzip.NewWriter(&gzipBuf)

		if _, err := gzipWriter.Write(ignitionJSON); err != nil {
			return fmt.Errorf("error gzip compressing ignition for %s: %w", hostname, err)
		}

		if err := gzipWriter.Close(); err != nil {
			return fmt.Errorf("error closing gzip writer for %s: %w", hostname, err)
		}

		// 4. Encode to base64.
		base64Encoded := base64.StdEncoding.EncodeToString(gzipBuf.Bytes())

		// 5. Find the node info for this hostname to get InstallDisk.
		var installDisk string

		for _, node := range nodes {
			if node.Hostname == hostname {
				installDisk = node.InstallDisk

				break
			}
		}

		if installDisk == "" {
			installDisk = defaultInstallDisk
		}

		// 6. Render bootstrap template using Go text/template.
		tmpl, err := texttemplate.New("bootstrap").Parse(string(bootstrapTemplateContent))
		if err != nil {
			return fmt.Errorf("error parsing bootstrap template: %w", err)
		}

		var renderedContent bytes.Buffer

		templateData := map[string]string{
			"Base64EncodedIgnition": base64Encoded,
			"InstallDisk":           installDisk,
		}

		if err := tmpl.Execute(&renderedContent, templateData); err != nil {
			return fmt.Errorf("error rendering bootstrap template for %s: %w", hostname, err)
		}

		// 7. Write bootstrap butane file.
		bootstrapPath := filepath.Join(bootstrapDir, entry.Name())
		if err := os.WriteFile(bootstrapPath, renderedContent.Bytes(), filePermissionUserReadWrite); err != nil {
			return fmt.Errorf("error writing bootstrap file %s: %w", bootstrapPath, err)
		}

		logrus.Debugf("Generated bootstrap template: %s", bootstrapPath)
	}

	logrus.Info("Bootstrap templates generated successfully")

	return nil
}

// CreateFolderStructure creates the directory structure declaratively.
func (i *Infrastructure) CreateFolderStructure() error {
	folders := []string{
		// Templates directory: editable source files.
		filepath.Join(i.Path, "templates", "butane", "install"),
		filepath.Join(i.Path, "templates", "butane", "bootstrap"),
		// Server directory: ready to serve via HTTP.
		filepath.Join(i.Path, "server", "ignition"),
		filepath.Join(i.Path, "server", "assets", "flatcar"),
		filepath.Join(i.Path, "server", "assets", "extensions"),
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

	for _, line := range strings.Split(content, "\n") {
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

// extractHostnameFromButane extracts the hostname from a Butane YAML document.
// It looks for the "inline:" value in the /etc/hostname file definition.
func extractHostnameFromButane(content string) string {
	lines := strings.Split(content, "\n")
	foundHostnamePath := false

	for i, line := range lines {
		// Look for the hostname file path.
		if foundHostnamePath {
			// Next line after "contents:" should have "inline:" with the hostname.
			if len(line) > 0 && (line[0] == ' ' || line[0] == '\t') {
				// Check if this line contains "inline:".
				if idx := strings.Index(line, "inline:"); idx >= 0 {
					// Extract the hostname after "inline:".
					hostname := line[idx+inlineKeywordLength:] // Skip "inline:".

					// Trim spaces.
					hostname = strings.TrimSpace(hostname)

					return hostname
				}
			}
		}

		// Look for "path: /etc/hostname".
		if idx := strings.Index(line, "path:"); idx >= 0 {
			if idx2 := strings.Index(line, "/etc/hostname"); idx2 >= 0 {
				// Found the hostname file definition, next "inline:" will have the hostname.
				foundHostnamePath = true

				// Also check if "contents:" is in the next few lines.
				for j := i + 1; j < i+maxLineLookAhead && j < len(lines); j++ {
					if idx3 := strings.Index(lines[j], "inline:"); idx3 >= 0 {
						hostname := lines[j][idx3+inlineKeywordLength:]
						hostname = strings.TrimSpace(hostname)

						return hostname
					}
				}
			}
		}
	}

	return ""
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
			return nil, fmt.Errorf("%w at index %d", ErrInvalidNodeMap, idx)
		}

		// Extract basic info.
		hostname, ok := nodeMap["hostname"].(string)
		if !ok || hostname == "" {
			return nil, fmt.Errorf("%w at index %d", ErrNodeHostnameRequired, idx)
		}

		macAddress, ok := nodeMap["macAddress"].(string)
		if !ok || macAddress == "" {
			return nil, fmt.Errorf("%w for node %s", ErrNodeMACRequired, hostname)
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

		// Extract arch (default to x86-64).
		arch := "x86-64"
		if archValue, ok := nodeMap["arch"].(string); ok && archValue != "" {
			arch = archValue
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
			Arch:           arch,
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
func (*Infrastructure) readSSHPublicKeys(sshConfig map[string]any) ([]string, error) {
	var (
		privateKeyPath string
		publicKeyPath  string
	)

	// 1. Determine private key path (with deprecation handling).
	// Prefer privateKeyPath over the deprecated keyPath.
	if newPrivKeyPath, ok := sshConfig["privateKeyPath"].(string); ok && newPrivKeyPath != "" {
		privateKeyPath = newPrivKeyPath
	} else if oldKeyPath, ok := sshConfig["keyPath"].(string); ok && oldKeyPath != "" {
		privateKeyPath = oldKeyPath

		logrus.Warn("ssh.keyPath is deprecated, please use ssh.privateKeyPath instead")
	} else {
		return nil, ErrSSHKeyPathMustBeSpecified
	}

	// 2. Determine public key path (with fallback).
	// If publicKeyPath is explicitly provided, use it.
	// Otherwise, derive it from privateKeyPath by appending ".pub".
	if pkPath, ok := sshConfig["publicKeyPath"].(string); ok && pkPath != "" {
		publicKeyPath = pkPath
	} else {
		publicKeyPath = privateKeyPath + ".pub"
	}

	// 3. Expand tilde (~) to home directory first.
	// Os.ExpandEnv() only expands $HOME and ${HOME}, not ~.
	if strings.HasPrefix(publicKeyPath, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("error getting home directory: %w", err)
		}

		publicKeyPath = filepath.Join(homeDir, publicKeyPath[2:])
	} else {
		// Expand environment variables (e.g., ${HOME}, $HOME).
		publicKeyPath = os.ExpandEnv(publicKeyPath)
	}

	// 4. Read the public key file.
	keyContent, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("error reading SSH public key from %s: %w", publicKeyPath, err)
	}

	// 5. Trim whitespace and validate.
	key := strings.TrimSpace(string(keyContent))
	if key == "" {
		return nil, fmt.Errorf("%w: %s", ErrSSHPublicKeyEmpty, publicKeyPath)
	}

	return []string{key}, nil
}

// determineNodeRole determines the role of a node based on its hostname.
//
//nolint:gocyclo,revive // Complex role determination logic, refactoring would reduce readability.
func (i *Infrastructure) determineNodeRole(hostname string, kubeConfig map[string]any) string {
	// 1. Check if in controlPlane.members[].
	if cpAny, ok := kubeConfig["controlPlane"]; ok {
		if cpMap, ok := cpAny.(map[string]any); ok {
			if membersAny, ok := cpMap["members"]; ok {
				if membersSlice, ok := membersAny.([]any); ok {
					for _, memberAny := range membersSlice {
						// Members are now objects with {hostname: "..."}.
						if memberMap, ok := memberAny.(map[string]any); ok {
							if memberHostname, ok := memberMap["hostname"].(string); ok && memberHostname == hostname {
								return roleControlPlane
							}
						}
						// Fallback for legacy string format (backward compatibility).
						if member, ok := memberAny.(string); ok && member == hostname {
							return roleControlPlane
						}
					}
				}
			}
		}
	}

	// 2. Check if in etcd.members[].
	if etcdAny, ok := kubeConfig[roleETCD]; ok {
		if etcdMap, ok := etcdAny.(map[string]any); ok {
			if membersAny, ok := etcdMap["members"]; ok {
				if membersSlice, ok := membersAny.([]any); ok {
					for _, memberAny := range membersSlice {
						// Members are objects with {hostname: "..."}.
						if memberMap, ok := memberAny.(map[string]any); ok {
							if memberHostname, ok := memberMap["hostname"].(string); ok && memberHostname == hostname {
								return roleETCD
							}
						}
						// Fallback for legacy string format (backward compatibility).
						if member, ok := memberAny.(string); ok && member == hostname {
							return roleETCD
						}
					}
				}
			}
		}
	}

	// 3. Check if in loadBalancers.members[].
	if lbAny, ok := kubeConfig["loadBalancers"]; ok {
		if lbMap, ok := lbAny.(map[string]any); ok {
			if membersAny, ok := lbMap["members"]; ok {
				if membersSlice, ok := membersAny.([]any); ok {
					for _, memberAny := range membersSlice {
						// Members are objects with {hostname: "..."}.
						if memberMap, ok := memberAny.(map[string]any); ok {
							if memberHostname, ok := memberMap["hostname"].(string); ok && memberHostname == hostname {
								return roleLoadBalancer
							}
						}
						// Fallback for legacy string format (backward compatibility).
						if member, ok := memberAny.(string); ok && member == hostname {
							return roleLoadBalancer
						}
					}
				}
			}
		}
	}

	// 4. Check if in nodeGroups[].nodes[].
	if groupsAny, ok := kubeConfig["nodeGroups"]; ok {
		if groupsSlice, ok := groupsAny.([]any); ok {
			for _, groupAny := range groupsSlice {
				if groupMap, ok := groupAny.(map[string]any); ok {
					if nodesAny, ok := groupMap["nodes"]; ok {
						if nodesSlice, ok := nodesAny.([]any); ok {
							for _, nodeAny := range nodesSlice {
								// Nodes are now objects with {hostname: "..."}.
								if nodeMap, ok := nodeAny.(map[string]any); ok {
									if nodeHostname, ok := nodeMap["hostname"].(string); ok && nodeHostname == hostname {
										return roleWorker
									}
								}
								// Fallback for legacy string format (backward compatibility).
								if node, ok := nodeAny.(string); ok && node == hostname {
									return roleWorker
								}
							}
						}
					}
				}
			}
		}
	}

	// 5. Default to worker if not found.
	//nolint:lll // Long warning message, but needs context for user.
	logrus.Warnf("Node %s not found in controlPlane, etcd, loadBalancers, or nodeGroups, defaulting to worker role", hostname)

	return roleWorker
}

// extractNetworkInfo extracts network configuration for a node.
func (*Infrastructure) extractNetworkInfo(networkAny any, hostname string) (networkInfo, error) {
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
		return networkInfo{}, fmt.Errorf("%w: %s", ErrNoEthernetInterfaceFound, hostname)
	}

	// Extract addresses (e.g., ["192.168.1.11/24"]).
	addressesAny, ok := firstInterface["addresses"]
	if !ok {
		return networkInfo{}, fmt.Errorf("%w: %s", ErrAddressesNotFound, hostname)
	}

	addressesSlice, ok := addressesAny.([]any)
	if !ok || len(addressesSlice) == 0 {
		return networkInfo{}, fmt.Errorf("%w: %s", ErrAddressesInvalid, hostname)
	}

	// Get first address.
	addressWithCIDR, ok := addressesSlice[0].(string)
	if !ok {
		return networkInfo{}, fmt.Errorf("%w: %s", ErrAddressNotString, hostname)
	}

	// Parse IP and netmask from CIDR notation (e.g., "192.168.1.11/24").
	parts := strings.Split(addressWithCIDR, "/")
	if len(parts) != networkAddressParts {
		return networkInfo{}, fmt.Errorf("%w: %s (expected format: IP/CIDR)", ErrInvalidCIDRFormat, addressWithCIDR)
	}

	ip := parts[0]
	netmask := parts[1]

	// Validate IP format using net.ParseIP from Go stdlib.
	if net.ParseIP(ip) == nil {
		return networkInfo{}, fmt.Errorf("%w: %s", ErrInvalidIPAddress, ip)
	}

	// Extract gateway.
	gateway, ok := firstInterface["gateway"].(string)
	if !ok || gateway == "" {
		return networkInfo{}, fmt.Errorf("%w: %s", ErrGatewayNotFound, hostname)
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

// generateNodeConfigs converts Butane YAML to Ignition JSON for a node.
func (i *Infrastructure) generateNodeConfigs(idx int, node nodeInfo) error {
	// 1. Read Butane template from templates/ directory.
	butanePath := filepath.Join(i.Path, "templates", "butane", "install", node.Hostname+".bu")

	// 2. Write Ignition to server/ directory (ready to serve via HTTP).
	// Normalize MAC address: replace colons with hyphens for URL-safe paths.
	normalizedMAC := strings.ReplaceAll(node.MAC, ":", "-")
	macDir := filepath.Join(i.Path, "server", "ignition", normalizedMAC)

	if err := os.MkdirAll(macDir, iox.FullPermAccess); err != nil {
		return fmt.Errorf("error creating directory for MAC %s: %w", normalizedMAC, err)
	}

	ignitionPath := filepath.Join(macDir, "ignition.json")

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
	if err := os.WriteFile(ignitionPath, ignitionJSON, filePermissionUserReadWrite); err != nil {
		return fmt.Errorf("error writing ignition file %s: %w", ignitionPath, err)
	}

	logrus.Infof("✓ Generated Ignition config for node %s", node.Hostname)

	return nil
}

// loadSysextVersions loads version mappings from immutable.yaml manifest.
// The immutable.yaml file is a centralized manifest containing all versioning
// information for Kubernetes, sysext packages, and Flatcar Container Linux.
// It should be located in fury-kubernetes-immutable-installer repository when ready.
func (i *Infrastructure) loadSysextVersions() error {
	kubeVersion := i.getKubernetesVersion()

	// Load immutable.yaml manifest from installer directory.
	manifestPath := filepath.Join(i.DistroPath, "installers", "immutable", "immutable.yaml")

	manifest, err := i.parseImmutableManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("error loading immutable manifest: %w", err)
	}

	// Get the Kubernetes release for the requested version.
	release, ok := manifest.Kubernetes[kubeVersion]
	if !ok {
		//nolint:err113 // Error includes version list for debugging context.
		return fmt.Errorf("kubernetes version %s not found in immutable manifest (available: %v)",
			kubeVersion, i.getAvailableVersions(manifest))
	}

	// Iterate through sysext array and extract versions by name.
	for _, sysext := range release.Sysext {
		switch sysext.Name {
		case "containerd":
			i.sysextVersions.ContainerdVersion = sysext.Version

		case "kubernetes":
			i.sysextVersions.KubernetesVersion = sysext.Version
			i.sysextVersions.KubernetesMajor = sysext.VersionMajorMinor

		case "keepalived":
			i.sysextVersions.KeepaliveVersion = sysext.Version

		case "etcd":
			i.sysextVersions.EtcdVersion = sysext.Version
		}
	}

	// Validate that required sysext packages were found.
	if i.sysextVersions.ContainerdVersion == "" {
		return fmt.Errorf("%w: %s", ErrContainerdSysextNotFound, kubeVersion)
	}

	if i.sysextVersions.KubernetesVersion == "" {
		return fmt.Errorf("%w: %s", ErrKubernetesSysextNotFound, kubeVersion)
	}

	logrus.Infof("Loaded sysext versions for Kubernetes %s", kubeVersion)
	logrus.Debugf("  containerd: %s", i.sysextVersions.ContainerdVersion)
	logrus.Debugf("  kubernetes: %s", i.sysextVersions.KubernetesVersion)

	return nil
}

// parseImmutableManifest parses the immutable.yaml manifest file.
// This manifest contains all versioning information for the Immutable installer.
func (*Infrastructure) parseImmutableManifest(path string) (*immutableManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading immutable manifest at %s: %w", path, err)
	}

	var manifest immutableManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("error parsing immutable manifest: %w", err)
	}

	// Validate that at least one Kubernetes version is defined.
	if len(manifest.Kubernetes) == 0 {
		return nil, ErrNoKubernetesVersions
	}

	return &manifest, nil
}

// getAvailableVersions returns a list of available Kubernetes versions from manifest.
func (*Infrastructure) getAvailableVersions(manifest *immutableManifest) []string {
	versions := make([]string, 0, len(manifest.Kubernetes))
	for version := range manifest.Kubernetes {
		versions = append(versions, version)
	}

	return versions
}

// getKubernetesVersion extracts the Kubernetes version from configuration.
func (i *Infrastructure) getKubernetesVersion() string {
	kubeConfig, ok := i.ConfigData["kubernetes"].(map[string]any)
	if !ok {
		// Use default from installer if not specified.
		return "1.33.4"
	}

	version, ok := kubeConfig["version"].(string)
	if !ok || version == "" {
		return "1.33.4"
	}

	return version
}

// assetDownloader wraps the HTTP client with asset-specific download logic.
type assetDownloader struct {
	client         netx.Client
	goGetterClient *netx.GoGetterClient // Direct reference to go-getter for file-specific downloads.
	assetsPath     string
}

// extractUsedArchitectures analyzes nodes and returns unique architectures.
func (*Infrastructure) extractUsedArchitectures(nodes []nodeInfo) []string {
	archMap := make(map[string]bool)
	for _, node := range nodes {
		archMap[node.Arch] = true
	}

	architectures := make([]string, 0, len(archMap))
	for arch := range archMap {
		architectures = append(architectures, arch)
	}

	logrus.Infof("Detected architectures in cluster: %v", architectures)

	return architectures
}

// downloadAssets downloads Flatcar boot artifacts and sysext packages
// for the architectures used in the cluster.
func (i *Infrastructure) downloadAssets(release kubernetesRelease, usedArchitectures []string) error {
	logrus.Info("Downloading Flatcar boot artifacts and sysext packages...")

	// Create HTTP client with caching.
	httpClient := netx.NewGoGetterClient()
	cachedClient := netx.WithLocalCache(
		httpClient,
		filepath.Join(i.Path, "..", "..", ".cache", "assets"),
	)

	downloader := &assetDownloader{
		client:         cachedClient,
		goGetterClient: httpClient, // Keep reference to unwrapped client for file-specific downloads.
		assetsPath:     filepath.Join(i.Path, "server", "assets"),
	}

	// Download Flatcar artifacts by architecture.
	if err := i.downloadFlatcarArtifacts(downloader, release.Flatcar, usedArchitectures); err != nil {
		return fmt.Errorf("error downloading Flatcar artifacts: %w", err)
	}

	// Download sysext packages by architecture.
	if err := i.downloadSysextPackages(downloader, release.Sysext, usedArchitectures); err != nil {
		return fmt.Errorf("error downloading sysext packages: %w", err)
	}

	logrus.Info("✓ Asset download completed successfully")

	return nil
}

// downloadFlatcarArtifacts downloads kernel, initrd, and image for each architecture.
func (*Infrastructure) downloadFlatcarArtifacts(
	downloader *assetDownloader,
	flatcar flatcarRelease,
	architectures []string,
) error {
	for _, arch := range architectures {
		archInfo, ok := flatcar.Arch[arch]
		if !ok {
			return fmt.Errorf("%w: %s", ErrFlatcarArtifactsNotFound, arch)
		}

		logrus.Infof("Downloading Flatcar %s artifacts for %s...", flatcar.Version, arch)

		// Create subdirectory by architecture: server/assets/flatcar/{arch}/.
		flatcarDir := filepath.Join(downloader.assetsPath, "flatcar", arch)
		if err := os.MkdirAll(flatcarDir, iox.FullPermAccess); err != nil {
			return fmt.Errorf("error creating directory %s: %w", flatcarDir, err)
		}

		// Download kernel.
		if err := downloader.downloadAndValidate(
			archInfo.Kernel.URL,
			filepath.Join(flatcarDir, archInfo.Kernel.Filename),
			archInfo.Kernel.SHA256,
		); err != nil {
			return fmt.Errorf("error downloading kernel for %s: %w", arch, err)
		}

		// Download initrd.
		if err := downloader.downloadAndValidate(
			archInfo.Initrd.URL,
			filepath.Join(flatcarDir, archInfo.Initrd.Filename),
			archInfo.Initrd.SHA256,
		); err != nil {
			return fmt.Errorf("error downloading initrd for %s: %w", arch, err)
		}

		// Download image.
		if err := downloader.downloadAndValidate(
			archInfo.Image.URL,
			filepath.Join(flatcarDir, archInfo.Image.Filename),
			archInfo.Image.SHA256,
		); err != nil {
			return fmt.Errorf("error downloading image for %s: %w", arch, err)
		}

		logrus.Infof("✓ Flatcar artifacts for %s downloaded successfully", arch)
	}

	return nil
}

// downloadSysextPackages downloads systemd-sysext packages for each architecture.
func (*Infrastructure) downloadSysextPackages(
	downloader *assetDownloader,
	sysextPackages []sysextPackage,
	architectures []string,
) error {
	extensionsDir := filepath.Join(downloader.assetsPath, "extensions")

	for _, pkg := range sysextPackages {
		logrus.Infof("Downloading %s sysext package (v%s)...", pkg.Name, pkg.Version)

		for _, arch := range architectures {
			archInfo, ok := pkg.Arch[arch]
			if !ok {
				logrus.Warnf("Sysext package %s not available for architecture %s, skipping", pkg.Name, arch)

				continue
			}

			// Naming convention: {name}-{version}-{arch}.raw.
			filename := fmt.Sprintf("%s-%s-%s.raw", pkg.Name, pkg.Version, arch)
			destPath := filepath.Join(extensionsDir, filename)

			if err := downloader.downloadAndValidate(
				archInfo.URL,
				destPath,
				archInfo.SHA256,
			); err != nil {
				return fmt.Errorf("error downloading %s for %s: %w", pkg.Name, arch, err)
			}
		}

		logrus.Infof("✓ %s sysext package downloaded successfully", pkg.Name)
	}

	return nil
}

// downloadAndValidate downloads a file and optionally validates its checksum SHA256.
func (ad *assetDownloader) downloadAndValidate(url, destPath, expectedSHA256 string) error {
	// Check if file already exists with valid checksum (idempotent).
	if ad.fileExistsAndValid(destPath, expectedSHA256) {
		logrus.Debugf("Skipping download, file already exists: %s", filepath.Base(destPath))

		return nil
	}

	// Download file using ClientModeFile to prevent directory creation.
	logrus.Debugf("Downloading %s", url)

	if err := ad.goGetterClient.DownloadWithMode(url, destPath, getter.ClientModeFile); err != nil {
		return fmt.Errorf("error downloading from %s: %w", url, err)
	}

	// Validate checksum if present.
	if expectedSHA256 != "" {
		if err := ad.validateChecksum(destPath, expectedSHA256); err != nil {
			// Remove corrupted file.
			_ = os.Remove(destPath)

			return fmt.Errorf("checksum validation failed for %s: %w", filepath.Base(destPath), err)
		}

		logrus.Debugf("✓ Checksum validated for %s", filepath.Base(destPath))
	}

	return nil
}

// fileExistsAndValid checks if file exists and has valid checksum.
func (ad *assetDownloader) fileExistsAndValid(path, expectedSHA256 string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}

	if expectedSHA256 == "" {
		// No checksum to validate, assume valid.
		return true
	}

	return ad.validateChecksum(path, expectedSHA256) == nil
}

// validateChecksum validates the SHA256 checksum of a file.
func (*assetDownloader) validateChecksum(path, expectedSHA256 string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("error opening file: %w", err)
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return fmt.Errorf("error computing checksum: %w", err)
	}

	actualSHA256 := hex.EncodeToString(hasher.Sum(nil))

	if actualSHA256 != expectedSHA256 {
		return fmt.Errorf("%w: expected %s, got %s", ErrChecksumMismatch, expectedSHA256, actualSHA256)
	}

	return nil
}
