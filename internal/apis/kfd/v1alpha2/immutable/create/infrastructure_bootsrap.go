package create

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	texttemplate "text/template"

	"github.com/hashicorp/go-getter"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	"github.com/sighupio/fury-distribution/pkg/apis/immutable/v1alpha2/public"
	"github.com/sighupio/furyctl/internal/tool/butane"
	iox "github.com/sighupio/furyctl/internal/x/io"
	"github.com/sighupio/furyctl/pkg/template"
	netx "github.com/sighupio/furyctl/pkg/x/net"
)

var (
	ErrNoKubernetesVersions      = errors.New("no kubernetes versions defined in immutable installer spec")
	ErrKubernetesVersionNotFound = errors.New("kubernetes version not found in immutable installer spec")
	ErrFlatcarArtifactsNotFound  = errors.New("flatcar artifacts not found for architecture")
	ErrButaneConversionFatal     = errors.New("butane conversion fatal errors")
	ErrButaneFatalErrors         = errors.New("butane translation has fatal errors")
)

type immutableManifest struct {
	Kubernetes map[string]assets `yaml:"kubernetes"`
}

// assets represents a Kubernetes version entry in immutable.yaml.
type assets struct {
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
	URL string `yaml:"url"`
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
}

// assetDownloader wraps the HTTP client with asset-specific download logic.
type assetDownloader struct {
	client         netx.Client
	goGetterClient *netx.GoGetterClient // Direct reference to go-getter for file-specific downloads.
	assetsPath     string
}

// getNodeRole determines the role of a node based on its hostname by checking the cluster configuration.
// It returns "controlplane", "loadbalancer", "etcd", or "worker".
func (i *Infrastructure) getNodeRole(node string) string {
	// There must be at least one control plane member.
	for _, controlPlaneNode := range i.furyctlConf.Spec.Kubernetes.ControlPlane.Members {
		if node == controlPlaneNode.Hostname {
			return "controlplane"
		}
	}

	if i.furyctlConf.Spec.Infrastructure.LoadBalancers != nil &&
		i.furyctlConf.Spec.Infrastructure.LoadBalancers.Members != nil {
		for _, loadBalancerNode := range i.furyctlConf.Spec.Infrastructure.LoadBalancers.Members {
			if node == loadBalancerNode.Hostname {
				return "loadbalancer"
			}
		}
	}

	if i.furyctlConf.Spec.Kubernetes.Etcd != nil && i.furyctlConf.Spec.Kubernetes.Etcd.Members != nil {
		for _, etcdNode := range i.furyctlConf.Spec.Kubernetes.Etcd.Members {
			if node == etcdNode.Hostname {
				return "etcd"
			}
		}
	}

	return "worker"
}

// Read the SSH public key file path specified in the configuration and return its content as a string.
func (i *Infrastructure) getSSHPublicKeyContent() (string, error) {
	var (
		sshPublicKeyPath string
		err              error
	)

	if i.furyctlConf.Spec.Infrastructure.Ssh.PublicKeyPath != nil {
		sshPublicKeyPath = *i.furyctlConf.Spec.Infrastructure.Ssh.PublicKeyPath
	} else {
		sshPublicKeyPath = *i.furyctlConf.Spec.Infrastructure.Ssh.PrivateKeyPath + ".pub"
	}

	sshPublicKeyPath = strings.Replace(sshPublicKeyPath, "~", os.Getenv("HOME"), 1)

	sshPublicKeyPath, err = filepath.Abs(sshPublicKeyPath)
	if err != nil {
		return "", fmt.Errorf("error getting absolute path for SSH public key file: %w", err)
	}

	sshPublicKey, err := os.ReadFile(sshPublicKeyPath)
	if err != nil {
		return "", fmt.Errorf("error reading SSH public key file: %w", err)
	}

	sshPublicKeyContent := strings.TrimSpace(string(sshPublicKey))

	return sshPublicKeyContent, nil
}

// parseImmutableInstallerSpec parses the immutable.yaml manifest file.
// This manifest contains all versioning information for the Immutable installer.
func (*Infrastructure) parseImmutableInstallerSpec(path string) (*immutableManifest, error) {
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

func (i *Infrastructure) getImmutableAssets() (assets, error) {
	kubeVersion := i.kfdManifest.Kubernetes.Immutable.Version

	immutableSpecPath := filepath.Join(i.Path, "..", "vendor", "installers", "immutable", "immutable.yaml")

	immutableInstallerSpec, err := i.parseImmutableInstallerSpec(immutableSpecPath)
	if err != nil {
		return assets{}, fmt.Errorf("error loading immutable installer specs for templates: %w", err)
	}

	immutableAssets, ok := immutableInstallerSpec.Kubernetes[kubeVersion]
	if !ok {
		return assets{}, fmt.Errorf("%w: %s", ErrKubernetesVersionNotFound, kubeVersion)
	}

	return immutableAssets, nil
}

// Render templates for the root of the server.
func (i *Infrastructure) renderRootTemplates() error {
	// Create data that will be passed to the template.
	cfg := template.Config{
		Data: map[string]map[any]any{
			"data": {
				"ipxeServerURL": i.furyctlConf.Spec.Infrastructure.IpxeServer.Url,
			},
		},
	}

	if err := i.CopyFromTemplate(
		cfg,
		"immutable-infrastructure",
		filepath.Join(i.paths.DistroPath, "templates", "infrastructure", "immutable", "server"),
		filepath.Join(i.Path, "server"),
		i.paths.ConfigPath,
	); err != nil {
		return fmt.Errorf("error copying from templates: %w", err)
	}

	logrus.Debug("boot.ipxe templates rendered from distribution")

	return nil
}

// Generate Butane files from distribution's templates and then convert them to ignition files.
func (i *Infrastructure) renderButaneTemplates() error {
	// 1. Load the full immutable manifest to pass all sysext info to templates.
	immutableAssets, err := i.getImmutableAssets()
	if err != nil {
		return fmt.Errorf("error getting immutable assets: %w", err)
	}

	// Convert sysext packages to template-friendly format.
	sysextData := make(map[any]any)

	for _, pkg := range immutableAssets.Sysext {
		pkgData := map[any]any{
			"name":    pkg.Name,
			"version": pkg.Version,
			"arch":    make(map[any]any),
		}

		// Add arch-specific info.
		for arch, archInfo := range pkg.Arch {
			archData := map[any]any{
				"url": archInfo.URL,
			}

			if archMap, ok := pkgData["arch"].(map[any]any); ok {
				archMap[arch] = archData
			}
		}

		sysextData[pkg.Name] = pkgData
	}

	// Convert Flatcar boot artifacts to template-friendly format.
	flatcarData := map[any]any{
		"version": immutableAssets.Flatcar.Version,
		"channel": immutableAssets.Flatcar.Channel,
		"arch":    make(map[any]any),
	}

	for arch, archInfo := range immutableAssets.Flatcar.Arch {
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

		if flatcarArchMap, ok := flatcarData["arch"].(map[any]any); ok {
			flatcarArchMap[arch] = artifactsData
		}
	}

	// Determine source path of templates in distribution folder.
	sourcePath := filepath.Join(
		i.paths.DistroPath,
		"templates",
		"infrastructure",
		"immutable",
		"butane",
	)

	// Use CopyFromTemplate to render templates.
	targetPath := filepath.Join(i.Path, "butane", "node-config")

	for _, node := range i.furyctlConf.Spec.Infrastructure.Nodes {
		nodeRole := i.getNodeRole(node.Hostname)
		normalizedMAC := strings.ReplaceAll(string(node.MacAddress), ":", "-")

		sshPublicKeyContent, err := i.getSSHPublicKeyContent()
		if err != nil {
			return fmt.Errorf("error getting SSH public key content: %w", err)
		}

		// Create target directory for this node's config ignition.
		err = os.MkdirAll(
			filepath.Join(sourcePath, "node-config", normalizedMAC),
			iox.FullPermAccess,
		)
		if err != nil {
			return fmt.Errorf("error creating target directory: %w", err)
		}

		// Copy helper file to the target folder so it is available for all node templates.
		err = iox.CopyFile(
			filepath.Join(sourcePath, "_helpers.tpl"),
			filepath.Join(sourcePath, "node-config", normalizedMAC, "_helpers.tpl"),
		)
		if err != nil {
			return fmt.Errorf("error copying template helper for node %s: %w", node.Hostname, err)
		}

		// Copy the role's butane template to the target path with the node-specific data.
		err = iox.CopyFile(
			filepath.Join(sourcePath, nodeRole+".bu.tpl"),
			filepath.Join(sourcePath, "node-config", normalizedMAC, node.Hostname+".bu.tpl"),
		)
		if err != nil {
			return fmt.Errorf("error copying butane template for node %s: %w", node.Hostname, err)
		}

		// Create data that will be passed to the template.
		cfg := template.Config{
			Data: map[string]map[any]any{
				"data": {
					"SSHUser":        i.furyctlConf.Spec.Infrastructure.Ssh.Username,
					"SSHPublicKey":   sshPublicKeyContent,
					"node":           node,
					"role":           nodeRole,
					"flatcarVersion": immutableAssets.Flatcar.Version,
					"ipxeServerURL":  i.furyctlConf.Spec.Infrastructure.IpxeServer.Url,
					"sysext":         sysextData,
					"flatcar":        flatcarData,
				},
			},
		}

		if err := i.CopyFromTemplate(
			cfg,
			"immutable-infrastructure",
			filepath.Join(sourcePath, "node-config", normalizedMAC),
			targetPath,
			i.paths.ConfigPath,
		); err != nil {
			return fmt.Errorf("error copying from templates: %w", err)
		}
	}

	// Generate flatcar-install templates that embed install ignition.
	if err := i.generateInstallFlatcarIgnitionFiles(); err != nil {
		return fmt.Errorf("error generating faltcar-install ignition files: %w", err)
	}

	return nil
}

// Generates butane files from distribution templates.
// The flatcar-install templates embed the node-config ignition (compressed and base64 encoded) and handle
// the initial PXE boot -> disk installation workflow.
// The node-config has the final node configuration.
func (i *Infrastructure) generateInstallFlatcarIgnitionFiles() error {
	nodeConfigButanesDir := filepath.Join(i.Path, "butane", "node-config")
	installButanesDir := filepath.Join(i.Path, "butane", "install")

	// Get path to the install-flatcar butane file template in the distribution.
	flatcarInstallButaneTemplatePath := filepath.Join(
		i.paths.DistroPath,
		"templates",
		"infrastructure",
		"immutable",
		"butane",
		"install-flatcar.bu.tpl",
	)

	// Check if template exists.
	if _, err := os.Stat(flatcarInstallButaneTemplatePath); err != nil {
		return fmt.Errorf("install-flatcar butante template not found at %s: %w", flatcarInstallButaneTemplatePath, err)
	}

	// Read flatcar-install template once.
	flatcarInstallTemplateContent, err := os.ReadFile(flatcarInstallButaneTemplatePath)
	if err != nil {
		return fmt.Errorf("error reading flat-car install butante template: %w", err)
	}

	// Create butane runner.
	runner := butane.NewRunner()
	runner.SetPretty(true)

	for _, node := range i.furyctlConf.Spec.Infrastructure.Nodes {
		nodeConfigPath := filepath.Join(nodeConfigButanesDir, node.Hostname+".bu")
		logrus.Debugf("Generating flatcar-install for %s", node.Hostname)

		// Read node-config butane file.
		butaneContent, err := os.ReadFile(nodeConfigPath)
		if err != nil {
			return fmt.Errorf("error reading node-config butane %s: %w", nodeConfigPath, err)
		}

		// 2. Convert node-config butane to ignition JSON.
		ignitionJSON, report, err := runner.ConvertWithReport(butaneContent)
		if err != nil {
			return fmt.Errorf("error converting %s to ignition: %w", nodeConfigPath, err)
		}

		// Check for fatal errors in report.
		if report.IsFatal() {
			return fmt.Errorf("%w for %s: %s", ErrButaneConversionFatal, node.Hostname, report.String())
		}

		// Log warnings if present.
		if len(report.Entries) > 0 {
			logrus.Warnf("Butane conversion warnings for %s: %s", node.Hostname, report.String())
		}

		// Compress with gzip.
		var gzipBuf bytes.Buffer
		gzipWriter := gzip.NewWriter(&gzipBuf)

		if _, err := gzipWriter.Write(ignitionJSON); err != nil {
			return fmt.Errorf("error gzip compressing ignition for %s: %w", node.Hostname, err)
		}

		if err := gzipWriter.Close(); err != nil {
			return fmt.Errorf("error closing gzip writer for %s: %w", node.Hostname, err)
		}

		// Encode node-config to base64.
		base64Encoded := base64.StdEncoding.EncodeToString(gzipBuf.Bytes())

		// Render flatcar-install butane template using Go text/template.
		tmpl, err := texttemplate.New("flatcar-install").Parse(string(flatcarInstallTemplateContent))
		if err != nil {
			return fmt.Errorf("error parsing flatcar-install butane template: %w", err)
		}

		var renderedContent bytes.Buffer

		sshPublicKeyContent, err := i.getSSHPublicKeyContent()
		if err != nil {
			return fmt.Errorf("error getting SSH public key content: %w", err)
		}

		templateData := map[string]string{
			"base64EncodedIgnition": base64Encoded,
			"installDisk":           string(node.Storage.InstallDisk),
			"hostname":              node.Hostname,
			"ipxeServerURL":         string(i.furyctlConf.Spec.Infrastructure.IpxeServer.Url),
			"sshUsername":           i.furyctlConf.Spec.Infrastructure.Ssh.Username,
			"sshPublicKey":          sshPublicKeyContent,
		}

		if err := tmpl.Execute(&renderedContent, templateData); err != nil {
			return fmt.Errorf("error rendering flatcar-install template for %s: %w", node.Hostname, err)
		}

		// Write flatcar-install butane file for this node.
		flatcarInstallIgnitionPath := filepath.Join(installButanesDir, node.Hostname+".bu")

		if err := os.MkdirAll(installButanesDir, iox.FullPermAccess); err != nil {
			return fmt.Errorf("error creating destination folder %s: %w", installButanesDir, err)
		}

		if err := os.WriteFile(flatcarInstallIgnitionPath, renderedContent.Bytes(), iox.FullRWPermAccess); err != nil {
			return fmt.Errorf("error writing flatcar-install file %s: %w", flatcarInstallIgnitionPath, err)
		}

		logrus.Debugf("Generated flatcar-install butane file for %s at: %s", node.Hostname, flatcarInstallIgnitionPath)
	}

	logrus.Info("Flatcar installation butane files generated successfully")

	return nil
}

// Convert Butane YAML to Ignition JSON for a node.
func convertButaneToIgnition(butanePath, ignitionPath string) error {
	// 1. Read Butane file.
	butaneContent, err := os.ReadFile(butanePath)
	if err != nil {
		return fmt.Errorf("error reading butane file %s: %w", butanePath, err)
	}

	// 2. Create Butane runner.
	runner := butane.NewRunner()
	runner.SetPretty(true)

	// 3. Convert Butane YAML to Ignition JSON.
	ignitionJSON, report, err := runner.ConvertWithReport(butaneContent)
	if err != nil {
		return fmt.Errorf("error converting butane to ignition: %w", err)
	}

	// 4. Check for fatal errors in report.
	if report.IsFatal() {
		return fmt.Errorf("%w: %s", ErrButaneFatalErrors, report.String())
	}

	// 5. Log warnings if present.
	if len(report.Entries) > 0 {
		logrus.Warnf("Butane conversion warnings: %s", report.String())
	}

	// 6. Write Ignition JSON.
	if err := os.WriteFile(ignitionPath, ignitionJSON, iox.FullRWPermAccess); err != nil {
		return fmt.Errorf("error writing ignition file %s: %w", ignitionPath, err)
	}

	return nil
}

// Convert butane files to ignition files.
func (i *Infrastructure) generateNodeIgnition(node public.SpecInfrastructureNode) error {
	// Normalize MAC address: replace colons with hyphens for URL-safe paths.
	normalizedMAC := strings.ReplaceAll(string(node.MacAddress), ":", "-")
	macDir := filepath.Join(i.Path, "server", "ignition", normalizedMAC)

	if err := os.MkdirAll(macDir, iox.FullPermAccess); err != nil {
		return fmt.Errorf("error creating directory for MAC %s: %w", normalizedMAC, err)
	}

	// 1. Read Butane template from templates/ directory.
	installFlatcarButanePath := filepath.Join(i.Path, "butane", "install", node.Hostname+".bu")
	nodeConfigurationButanePath := filepath.Join(i.Path, "butane", "node-config", node.Hostname+".bu")

	installFlatcarIgnitionPath := filepath.Join(macDir, "install-flatcar.json")
	nodeConfigurationIgnitionPath := filepath.Join(macDir, "node-configuration.json")

	// 2. Convert Butane to Ignition for both install and node configuration.
	if err := convertButaneToIgnition(installFlatcarButanePath, installFlatcarIgnitionPath); err != nil {
		return fmt.Errorf("error generating install flatcar ignition for node %s: %w", node.Hostname, err)
	}

	if err := convertButaneToIgnition(nodeConfigurationButanePath, nodeConfigurationIgnitionPath); err != nil {
		return fmt.Errorf("error generating node configuration ignition for node %s: %w", node.Hostname, err)
	}

	logrus.Debugf("Generated ignition files for node %s", node.Hostname)

	return nil
}

// Analyze nodes and returns unique architectures.
func (i *Infrastructure) extractUsedArchitectures() []string {
	archMap := make(map[string]bool)
	for _, node := range i.furyctlConf.Spec.Infrastructure.Nodes {
		archMap[string(node.Arch)] = true
	}

	architectures := make([]string, 0, len(archMap))
	for arch := range archMap {
		architectures = append(architectures, arch)
	}

	logrus.Debugf("Detected architectures in cluster: %v", architectures)

	return architectures
}

// Download Flatcar boot artifacts and sysext packages for the architectures used in the cluster.
func (i *Infrastructure) downloadAssets(usedArchitectures []string) error {
	logrus.Info("Downloading Flatcar boot artifacts and sysext packages...")

	assets, err := i.getImmutableAssets()
	if err != nil {
		return fmt.Errorf("error getting immutable assets: %w", err)
	}

	// Create HTTP client with caching.
	// FIXME: path to cache should not be calculated this way.
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
	if err := i.downloadFlatcarArtifacts(downloader, assets.Flatcar, usedArchitectures); err != nil {
		return fmt.Errorf("error downloading Flatcar artifacts: %w", err)
	}

	// Download sysext packages by architecture.
	if err := i.downloadSysextPackages(downloader, assets.Sysext, usedArchitectures); err != nil {
		return fmt.Errorf("error downloading sysext packages: %w", err)
	}

	logrus.Info("Assets download completed successfully")

	return nil
}

// Downloads kernel, initrd, and image for each architecture.
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
		); err != nil {
			return fmt.Errorf("error downloading kernel for %s: %w", arch, err)
		}

		// Download initrd.
		if err := downloader.downloadAndValidate(
			archInfo.Initrd.URL,
			filepath.Join(flatcarDir, archInfo.Initrd.Filename),
		); err != nil {
			return fmt.Errorf("error downloading initrd for %s: %w", arch, err)
		}

		// Download image.
		if err := downloader.downloadAndValidate(
			archInfo.Image.URL,
			filepath.Join(flatcarDir, archInfo.Image.Filename),
		); err != nil {
			return fmt.Errorf("error downloading image for %s: %w", arch, err)
		}

		logrus.Infof("Flatcar artifacts for %s downloaded successfully", arch)
	}

	return nil
}

// Download systemd-sysext packages for each architecture.
func (*Infrastructure) downloadSysextPackages(
	downloader *assetDownloader,
	sysextPackages []sysextPackage,
	architectures []string,
) error {
	extensionsDir := filepath.Join(downloader.assetsPath, "extensions")

	for _, pkg := range sysextPackages {
		logrus.Infof("Downloading %s sysext package %s...", pkg.Name, pkg.Version)

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
			); err != nil {
				return fmt.Errorf("error downloading %s for %s: %w", pkg.Name, arch, err)
			}
		}

		logrus.Infof("%s sysext package downloaded successfully", pkg.Name)
	}

	return nil
}

// Download a file to the specified destination path.
func (ad *assetDownloader) downloadAndValidate(url, destPath string) error {
	// Check if file already exists (idempotent).
	// FIXME: this is a very simple check that doesn't validate file integrity (e.g. with checksums).
	// We should enhance this in the future.
	// Maybe we could reuse something from the dependency manager that already handles caching and validation?
	if ad.fileExistsAndValid(destPath) {
		logrus.Debugf("Skipping download, file already exists: %s", filepath.Base(destPath))

		return nil
	}

	// Download file using ClientModeFile to prevent directory creation.
	logrus.Debugf("Downloading %s", url)

	if err := ad.goGetterClient.DownloadWithMode(url, destPath, getter.ClientModeFile); err != nil {
		return fmt.Errorf("error downloading from %s: %w", url, err)
	}

	return nil
}

// Checks if file exists.
func (*assetDownloader) fileExistsAndValid(path string) bool {
	_, err := os.Stat(path)

	return !os.IsNotExist(err)
}

// Bootstrap Flatcar nodes by:
// - Downloading the Flatcar image and prepare the assets for the installer defined in immutable.yaml.
// - Starting a server to serve the assets to the installer.
func (i *Infrastructure) BootstrapNodes() error {
	logrus.Debug("Bootstrapping nodes...")

	if err := i.renderRootTemplates(); err != nil {
		return fmt.Errorf("error rendering root templates: %w", err)
	}

	// Render Butane templates from distribution.
	if err := i.renderButaneTemplates(); err != nil {
		return fmt.Errorf("error rendering butane templates: %w", err)
	}

	// Post-process: convert .bu to .ign for each node.
	for _, node := range i.furyctlConf.Spec.Infrastructure.Nodes {
		if err := i.generateNodeIgnition(node); err != nil {
			return fmt.Errorf("error generating configs for node %s: %w", node.Hostname, err)
		}
	}

	// Download assets (Flatcar boot artifacts and sysext packages) for the architectures used in the cluster.
	usedArchitectures := i.extractUsedArchitectures()
	if err := i.downloadAssets(usedArchitectures); err != nil {
		return fmt.Errorf("error downloading assets: %w", err)
	}

	return nil
}
