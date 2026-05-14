// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package clusterinfo

import "time"

// Info holds the complete information about an SD cluster and its status.
type Info struct {
	ClusterName             string          `json:"clusterName"                  yaml:"clusterName"`
	SDVersion               string          `json:"sdVersion"                    yaml:"sdVersion"`
	SDKind                  string          `json:"sdKind"                       yaml:"sdKind"`
	SDInstallerVersion      string          `json:"sdInstallerVersion,omitempty" yaml:"sdInstallerVersion,omitempty"`
	SDUpgradePaths          []string        `json:"sdUpgradePaths"               yaml:"sdUpgradePaths"`
	SDOngoingUpgrade        *OngoingUpgrade `json:"sdOngoingUpgrade,omitempty"   yaml:"sdOngoingUpgrade,omitempty"`
	KubernetesVersion       string          `json:"kubernetesVersion"            yaml:"kubernetesVersion"`
	EtcdTopology            string          `json:"etcdTopology,omitempty"       yaml:"etcdTopology,omitempty"`
	LastConfigurationChange time.Time       `json:"lastConfigurationChange"      yaml:"lastConfigurationChange"`
	CustomPatchesPresent    bool            `json:"customPatchesPresent"         yaml:"customPatchesPresent"`
	Modules                 []ModuleInfo    `json:"modules"                      yaml:"modules"`
	Plugins                 *PluginsInfo    `json:"plugins,omitempty"            yaml:"plugins,omitempty"`
	Nodes                   *NodesSummary   `json:"nodes,omitempty"              yaml:"nodes,omitempty"`
}

// OngoingUpgrade describes an in-progress or failed cluster upgrade.
type OngoingUpgrade struct {
	Status string `json:"status" yaml:"status"`
	Phase  string `json:"phase"  yaml:"phase"`
}

// ModuleInfo describes an SD module with its installed version and type.
type ModuleInfo struct {
	Name    string `json:"name"    yaml:"name"`
	Version string `json:"version" yaml:"version"`
	Type    string `json:"type"    yaml:"type"`
}

// PluginsInfo groups plugin names by their type.
type PluginsInfo struct {
	Kustomize []string `json:"kustomize,omitempty" yaml:"kustomize,omitempty"`
	Helm      []string `json:"helm,omitempty"      yaml:"helm,omitempty"`
}

// NodeRoleGroup holds aggregate capacity for nodes sharing the same role.
type NodeRoleGroup struct {
	Role     string  `json:"role"     yaml:"role"`
	Quantity int     `json:"quantity" yaml:"quantity"`
	VCPU     int64   `json:"vcpu"     yaml:"vcpu"`
	RAMGb    float64 `json:"ramGb"    yaml:"ramGb"`
}

// NodesSummary groups node capacity by role with aggregate totals.
type NodesSummary struct {
	Roles  []NodeRoleGroup `json:"roles"  yaml:"roles"`
	Totals NodeTotals      `json:"totals" yaml:"totals"`
}

// NodeTotals holds the aggregate capacity across all nodes.
type NodeTotals struct {
	Quantity int     `json:"quantity" yaml:"quantity"`
	VCPU     int64   `json:"vcpu"     yaml:"vcpu"`
	RAMGb    float64 `json:"ramGb"    yaml:"ramGb"`
}
