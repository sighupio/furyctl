// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
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
	DisableDefaultSNAT             bool   `yaml:"disableDefaultSNAT"`

	SubNetworks  []string          `yaml:"subnetworks"`
	DMZCIDRRange DMZCIDRRange      `yaml:"dmzCIDRRange"`
	SSHPublicKey string            `yaml:"sshPublicKey"`
	NodePools    []GKENodePool     `yaml:"nodePools"`
	Tags         map[string]string `yaml:"tags"`
	Region       string            `yaml:"region"`
	Project      string            `yaml:"project"`
}

// GKENodePool represent a node pool configuration
type GKENodePool struct {
	Name                    string              `yaml:"name"`
	Version                 string              `yaml:"version"`
	MinSize                 int                 `yaml:"minSize"`
	MaxSize                 int                 `yaml:"maxSize"`
	InstanceType            string              `yaml:"instanceType"`
	OS                      string              `yaml:"os"`
	MaxPods                 int                 `yaml:"maxPods"`
	SpotInstance            bool                `yaml:"spotInstance"`
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
