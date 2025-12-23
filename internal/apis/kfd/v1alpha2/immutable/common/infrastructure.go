// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package common

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sighupio/furyctl/internal/cluster"
	iox "github.com/sighupio/furyctl/internal/x/io"
)

const (
	flatcarVersion = "4206.0.0"

	// NetworkAddressParts is the expected number of parts in a network address.
	networkAddressParts = 2
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

	for idx, node := range nodes {
		if err := i.generateNodeConfigs(idx, node); err != nil {
			return fmt.Errorf("error generating configs for node %s: %w", node.Hostname, err)
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
