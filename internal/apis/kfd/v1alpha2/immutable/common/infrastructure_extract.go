// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package common

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// extractNodes extracts and processes node information from config.
func (i *Infrastructure) extractNodes() ([]nodeInfo, error) {
	specConfig, ok := i.ConfigData["spec"].(map[string]any)
	if !ok {
		return nil, ErrInfraConfigNotFound
	}

	infraConfig, ok := specConfig["infrastructure"].(map[string]any)
	if !ok {
		return nil, ErrInfraConfigNotFound
	}

	ipxeURL, err := extractIPXEServerURL(infraConfig)
	if err != nil {
		return nil, err
	}

	sshUser, sshKeys, err := extractSSHConfig(infraConfig)
	if err != nil {
		return nil, err
	}

	nodesArray, err := extractNodesArray(infraConfig)
	if err != nil {
		return nil, err
	}

	cpHostnames, err := i.extractControlPlaneHostnames()
	if err != nil {
		return nil, err
	}

	nodes := make([]nodeInfo, 0, len(nodesArray))

	for _, node := range nodesArray {
		nodeMap, ok := node.(map[string]any)
		if !ok {
			continue
		}

		processedNode, err := processNode(nodeMap, cpHostnames, ipxeURL, sshUser, sshKeys)
		if err != nil {
			return nil, err
		}

		nodes = append(nodes, processedNode)
	}

	return nodes, nil
}

// extractIPXEServerURL extracts the IPXE server URL from infrastructure config.
func extractIPXEServerURL(infraConfig map[string]any) (string, error) {
	ipxeServer, ok := infraConfig["ipxeServer"].(map[string]any)
	if !ok {
		return "", ErrIPXEServerNotFound
	}

	ipxeURL, ok := ipxeServer["url"].(string)
	if !ok {
		return "", ErrIPXEServerURLNotFound
	}

	return ipxeURL, nil
}

// extractSSHConfig extracts SSH configuration from infrastructure config.
func extractSSHConfig(infraConfig map[string]any) (string, []string, error) {
	sshConfig, ok := infraConfig["ssh"].(map[string]any)
	if !ok {
		return "", nil, ErrSSHConfigNotFound
	}

	sshUser, ok := sshConfig["username"].(string)
	if !ok {
		sshUser = defaultSSHUser // Use default from constants.
	}

	sshKeyPath, ok := sshConfig["keyPath"].(string)
	if !ok {
		return "", nil, ErrSSHKeyPathNotFound
	}

	sshKeys, err := readSSHKey(sshKeyPath)
	if err != nil {
		return "", nil, fmt.Errorf("error reading SSH key: %w", err)
	}

	return sshUser, sshKeys, nil
}

// extractNodesArray extracts the nodes array from infrastructure config.
func extractNodesArray(infraConfig map[string]any) ([]any, error) {
	nodesArray, ok := infraConfig["nodes"].([]any)
	if !ok {
		return nil, ErrNodesNotFound
	}

	return nodesArray, nil
}

// extractControlPlaneHostnames extracts control plane hostnames from kubernetes config.
func (i *Infrastructure) extractControlPlaneHostnames() (map[string]bool, error) {
	specConfig, ok := i.ConfigData["spec"].(map[string]any)
	if !ok {
		return nil, ErrKubeConfigNotFound
	}

	kubeConfig, ok := specConfig["kubernetes"].(map[string]any)
	if !ok {
		return nil, ErrKubeConfigNotFound
	}

	cpConfig, ok := kubeConfig["controlPlane"].(map[string]any)
	if !ok {
		return nil, ErrControlPlaneNotFound
	}

	cpMembers, ok := cpConfig["members"].([]any)
	if !ok {
		return nil, ErrControlMembersNotFound
	}

	cpHostnames := make(map[string]bool, len(cpMembers))

	for _, member := range cpMembers {
		memberMap, ok := member.(map[string]any)
		if !ok {
			continue
		}

		if hostname, ok := memberMap["hostname"].(string); ok {
			cpHostnames[hostname] = true
		}
	}

	return cpHostnames, nil
}

// processNode processes a single node from the config.
func processNode(
	nodeMap map[string]any,
	cpHostnames map[string]bool,
	ipxeURL string,
	sshUser string,
	sshKeys []string,
) (nodeInfo, error) {
	hostname, _ := nodeMap["hostname"].(string) //nolint:errcheck,revive // Optional field.
	mac, _ := nodeMap["macAddress"].(string)    //nolint:errcheck,revive // Optional field.

	installDisk, err := extractStorageConfig(nodeMap, hostname)
	if err != nil {
		return nodeInfo{}, err
	}

	netInfo, err := extractNetworkInfo(nodeMap, hostname)
	if err != nil {
		return nodeInfo{}, err
	}

	role := determineNodeRole(hostname, nodeMap, cpHostnames)

	return nodeInfo{
		Hostname:       hostname,
		MAC:            mac,
		IP:             netInfo.IP,
		Gateway:        netInfo.Gateway,
		DNS:            netInfo.DNS,
		Netmask:        netInfo.Netmask,
		Role:           role,
		InstallDisk:    installDisk,
		SSHUser:        sshUser,
		SSHKeys:        sshKeys,
		IPXEServerURL:  ipxeURL,
		FlatcarVersion: "", // Will be set from Infrastructure config.
	}, nil
}

// extractStorageConfig extracts storage configuration from a node.
func extractStorageConfig(nodeMap map[string]any, hostname string) (string, error) {
	storage, ok := nodeMap["storage"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrStorageNotFound, hostname)
	}

	installDisk, ok := storage["installDisk"].(string)
	if !ok {
		installDisk = defaultInstallDisk // Use default from constants.
	}

	return installDisk, nil
}

// extractNetworkInfo extracts network information from a node.
func extractNetworkInfo(
	nodeMap map[string]any,
	hostname string,
) (networkInfo, error) {
	network, ok := nodeMap["network"].(map[string]any)
	if !ok {
		return networkInfo{}, fmt.Errorf("%w: %s", ErrNetworkNotFound, hostname)
	}

	ethernets, ok := network["ethernets"].(map[string]any)
	if !ok {
		return networkInfo{}, fmt.Errorf("%w: %s", ErrNetworkEthersNotFound, hostname)
	}

	var netInfo networkInfo

	for _, ethConfig := range ethernets {
		ethMap, ok := ethConfig.(map[string]any)
		if !ok {
			continue
		}

		// Get addresses.
		addresses, ok := ethMap["addresses"].([]any)
		if ok && len(addresses) > 0 {
			addr, ok := addresses[0].(string)
			if ok {
				parts := strings.Split(addr, "/")
				if len(parts) == networkAddressParts {
					netInfo.IP = parts[0]
					netInfo.Netmask = parts[1]
				}
			}
		}

		// Get gateway.
		if gw, ok := ethMap["gateway"].(string); ok {
			netInfo.Gateway = gw
		}

		// Get nameservers.
		if ns, ok := ethMap["nameservers"].(map[string]any); ok {
			if addrs, ok := ns["addresses"].([]any); ok && len(addrs) > 0 {
				if dnsAddr, ok := addrs[0].(string); ok {
					netInfo.DNS = dnsAddr
				}
			}
		}

		if netInfo.IP != "" {
			break
		}
	}

	return netInfo, nil
}

// determineNodeRole determines the role of a node.
func determineNodeRole(
	hostname string,
	nodeMap map[string]any,
	cpHostnames map[string]bool,
) string {
	if cpHostnames[hostname] {
		return "controlplane"
	}

	if hasVirtualIP(nodeMap) {
		return "loadbalancer"
	}

	return "worker"
}

// hasVirtualIP checks if a node has a virtual IP (load balancer).
func hasVirtualIP(nodeMap map[string]any) bool {
	network, ok := nodeMap["network"].(map[string]any)
	if !ok {
		return false
	}

	ethernets, ok := network["ethernets"].(map[string]any)
	if !ok {
		return false
	}

	for _, ethConfig := range ethernets {
		ethMap, ok := ethConfig.(map[string]any)
		if !ok {
			continue
		}

		addresses, ok := ethMap["addresses"].([]any)
		if !ok || len(addresses) < 2 {
			continue
		}

		// Check if any address is a /32 (VIP).
		for _, addr := range addresses {
			addrStr, ok := addr.(string)
			if ok && strings.HasSuffix(addrStr, "/32") {
				return true
			}
		}
	}

	return false
}

// readSSHKey reads SSH public key from file.
func readSSHKey(keyPath string) ([]string, error) {
	// Expand home directory.
	if strings.HasPrefix(keyPath, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("error getting home directory: %w", err)
		}

		keyPath = filepath.Join(home, keyPath[2:])
	}

	// Read public key (add .pub extension).
	pubKeyPath := keyPath + ".pub"

	content, err := os.ReadFile(pubKeyPath)
	if err != nil {
		return nil, fmt.Errorf("error reading SSH public key: %w", err)
	}

	key := strings.TrimSpace(string(content))

	return []string{key}, nil
}
