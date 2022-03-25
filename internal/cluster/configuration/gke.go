// Copyright (c) 2022 SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package configuration

// GKE represents the configuration spec of a GKE Cluster
type GKE struct {
	Version string `yaml:"version"`
	Network string `yaml:"network"`

	NetworkProjectID               string `yaml:"networkProjectID"`
	ControlPlaneCIDR               string `yaml:"controlPlaneCIDR"`
	AdditionalFirewallRules        bool   `yaml:"additionalFirewallRules"`
	AdditionalClusterFirewallRules bool   `yaml:"additionalClusterFirewallRules"`
	DisalbeDefaultSNAT             bool   `yaml:"disalbeDefaultSNAT"`

	SubNetworks  []string          `yaml:"subnetworks"`
	DMZCIDRRange DMZCIDRRange      `yaml:"dmzCIDRRange"`
	SSHPublicKey string            `yaml:"sshPublicKey"`
	NodePools    []GKENodePool     `yaml:"nodePools"`
	Tags         map[string]string `yaml:"tags"`
}

// GKENodePool represent a node pool configuration
type GKENodePool struct {
	Name                    string              `yaml:"name"`
	Version                 string              `yaml:"version"`
	MinSize                 int                 `yaml:"minSize"`
	MaxSize                 int                 `yaml:"maxSize"`
	InstanceType            string              `yaml:"instanceType"`
	OS                      string              `yaml:"os"`
	SpotInstance            bool                `yaml:"spotInstance"`
	MaxPods                 int                 `yaml:"maxPods"`
	VolumeSize              int                 `yaml:"volumeSize"`
	Labels                  map[string]string   `yaml:"labels"`
	Taints                  []string            `yaml:"taints"`
	SubNetworks             []string            `yaml:"subnetworks"`
	Tags                    map[string]string   `yaml:"tags"`
	AdditionalFirewallRules []GKENodePoolFwRule `yaml:"additionalFirewallRules"`
}

// GKENodePoolFwRule represent an additional firewall rule to add to a specific node pool in the cluster
type GKENodePoolFwRule struct {
	Name      string            `yaml:"name"`
	Direction string            `yaml:"direction"`
	CIDRBlock string            `yaml:"cidrBlock"`
	Protocol  string            `yaml:"protocol"`
	Ports     string            `yaml:"ports"`
	Tags      map[string]string `yaml:"tags"`
}
