// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package common

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"

	"github.com/coreos/butane/config"
	"github.com/coreos/butane/config/common"

	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/immutable/templates"
	iox "github.com/sighupio/furyctl/internal/x/io"
)

// generateNodeConfigs generates all config files for a node.
func (i *Infrastructure) generateNodeConfigs(id int, node nodeInfo) error {
	// 1. Render install Butane template.
	templatePath := getButaneTemplatePath(node.Role)

	installButane, err := renderInstallButane(templatePath, id, node)
	if err != nil {
		return fmt.Errorf("error rendering install butane: %w", err)
	}

	// 2. Write install Butane file.
	installButanePath := filepath.Join(i.Path, "butane", "install", node.Hostname+".bu")

	if err := os.WriteFile(installButanePath, installButane, iox.FullPermAccess); err != nil {
		return fmt.Errorf("error writing install butane: %w", err)
	}

	// 3. Convert install Butane to Ignition.
	installIgnition, err := butaneToIgnition(installButane)
	if err != nil {
		return fmt.Errorf("error converting install butane to ignition: %w", err)
	}

	// 4. Write install Ignition file.
	installIgnitionPath := filepath.Join(i.Path, "ignition", "install", node.Hostname+".ign")

	if err := os.WriteFile(installIgnitionPath, installIgnition, iox.FullPermAccess); err != nil {
		return fmt.Errorf("error writing install ignition: %w", err)
	}

	// 5. Generate bootstrap Butane (embeds install ignition).
	bootstrapButane, err := generateBootstrapButane(node, installIgnition)
	if err != nil {
		return fmt.Errorf("error generating bootstrap butane: %w", err)
	}

	// 6. Write bootstrap Butane file.
	bootstrapButanePath := filepath.Join(i.Path, "butane", "bootstrap", node.Hostname+".bu")

	if err := os.WriteFile(bootstrapButanePath, []byte(bootstrapButane), iox.FullPermAccess); err != nil {
		return fmt.Errorf("error writing bootstrap butane: %w", err)
	}

	// 7. Convert bootstrap Butane to Ignition.
	bootstrapIgnition, err := butaneToIgnition([]byte(bootstrapButane))
	if err != nil {
		return fmt.Errorf("error converting bootstrap butane to ignition: %w", err)
	}

	// 8. Write bootstrap Ignition file.
	bootstrapIgnitionPath := filepath.Join(i.Path, "ignition", node.Hostname+".ign")

	if err := os.WriteFile(bootstrapIgnitionPath, bootstrapIgnition, iox.FullPermAccess); err != nil {
		return fmt.Errorf("error writing bootstrap ignition: %w", err)
	}

	return nil
}

// getButaneTemplatePath returns the template path for a node role.
func getButaneTemplatePath(role string) string {
	switch role {
	case "controlplane":
		return "butane/controlplane.bu.tmpl"

	case "loadbalancer":
		return "butane/loadbalancer.bu.tmpl"

	case "worker":
		return "butane/worker.bu.tmpl"

	default:
		return "butane/worker.bu.tmpl"
	}
}

// renderInstallButane renders the install Butane template for a node.
func renderInstallButane(templatePath string, id int, node nodeInfo) ([]byte, error) {
	nodeData := templates.NodeData{
		ID:             id,
		Hostname:       node.Hostname,
		IP:             node.IP,
		MAC:            node.MAC,
		Role:           node.Role,
		Netmask:        node.Netmask,
		Gateway:        node.Gateway,
		DNS:            node.DNS,
		SSHUser:        node.SSHUser,
		SSHKeys:        node.SSHKeys,
		FlatcarVersion: node.FlatcarVersion,
		InstallDisk:    node.InstallDisk,
		IPXEServerURL:  node.IPXEServerURL,
	}

	butaneBytes, err := templates.RenderButaneTemplate(templatePath, nodeData)
	if err != nil {
		return nil, fmt.Errorf("template rendering failed: %w", err)
	}

	return butaneBytes, nil
}

// butaneToIgnition converts Butane config to Ignition using the Go package.
func butaneToIgnition(butaneConfig []byte) ([]byte, error) {
	options := common.TranslateBytesOptions{
		Pretty: false,
	}

	ignition, report, err := config.TranslateBytes(butaneConfig, options)
	if err != nil {
		return nil, fmt.Errorf("butane translation failed: %w\n%s", err, report.String())
	}

	if report.IsFatal() {
		return nil, fmt.Errorf("%w:\n%s", ErrButaneFatalErrors, report.String())
	}

	return ignition, nil
}

// generateBootstrapButane generates the bootstrap Butane config.
func generateBootstrapButane(node nodeInfo, installIgnition []byte) (string, error) {
	base64Encoded, err := compressAndEncode(installIgnition)
	if err != nil {
		return "", err
	}

	data := templates.NodeData{
		InstallDisk:           node.InstallDisk,
		Base64EncodedIgnition: base64Encoded,
	}

	rendered, err := templates.RenderButaneTemplate("butane/bootstrap.bu.tmpl", data)
	if err != nil {
		return "", fmt.Errorf("error rendering bootstrap template: %w", err)
	}

	return string(rendered), nil
}

func compressAndEncode(data []byte) (string, error) {
	var buf bytes.Buffer

	gzWriter := gzip.NewWriter(&buf)

	if _, err := gzWriter.Write(data); err != nil {
		return "", fmt.Errorf("error compressing data: %w", err)
	}

	if err := gzWriter.Close(); err != nil {
		return "", fmt.Errorf("error closing gzip writer: %w", err)
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}
