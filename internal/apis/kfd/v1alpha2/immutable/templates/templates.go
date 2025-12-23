// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package templates

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"
)

//go:embed butane/*.bu.tmpl
var templatesFS embed.FS

// NodeData contains all the data needed to render a node's configuration.
type NodeData struct {
	// Node identification.
	ID       int
	Hostname string
	IP       string
	MAC      string
	// Loadbalancer, controlplane, worker.
	Role string

	// Network configuration.
	Netmask string
	Gateway string
	DNS     string

	// SSH configuration.
	SSHUser string
	SSHKeys []string

	// Flatcar configuration.
	FlatcarVersion string
	InstallDisk    string

	// IPXE server configuration.
	IPXEServerURL string

	// Ignition (for bootstrap butane generation).
	// Base64 encoded (gzipped) install ignition.
	Base64EncodedIgnition string
}

// RenderButaneTemplate renders a Butane template with the provided node data.
func RenderButaneTemplate(templatePath string, data NodeData) ([]byte, error) {
	tmplContent, err := templatesFS.ReadFile(templatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read template %s: %w", templatePath, err)
	}

	tmpl, err := template.New(templatePath).Parse(string(tmplContent))
	if err != nil {
		return nil, fmt.Errorf("failed to parse template %s: %w", templatePath, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("failed to execute template %s: %w", templatePath, err)
	}

	return buf.Bytes(), nil
}
