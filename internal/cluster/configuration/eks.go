// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package configuration

// EKS represents the configuration spec of a EKS Cluster
type EKS struct {
	Version                          string            `yaml:"version"`
	Network                          string            `yaml:"network"`
	SubNetworks                      []string          `yaml:"subnetworks"`
	DMZCIDRRange                     DMZCIDRRange      `yaml:"dmzCIDRRange"`
	SSHPublicKey                     string            `yaml:"sshPublicKey"`
	NodePools                        []EKSNodePool     `yaml:"nodePools"`
	NodePoolsLaunchKind              string            `yaml:"nodePoolsLaunchKind"`
	Tags                             map[string]string `yaml:"tags"`
	Auth                             EKSAuth           `yaml:"auth"`
	LogRetentionDays                 int               `yaml:"logRetentionDays"`
	ClusterEndpointPublicAccess      bool              `yaml:"clusterEndpointPublicAccess"`
	ClusterEndpointPublicAccessCidrs []string          `yaml:"clusterEndpointPublicAccessCidrs"`
}

// EKSAuth represent a auth structure
type EKSAuth struct {
	AdditionalAccounts []string      `yaml:"additionalAccounts"`
	Users              []EKSAuthData `yaml:"users"`
	Roles              []EKSAuthData `yaml:"roles"`
}

// EKSAuthData represent a auth structure
type EKSAuthData struct {
	Username string   `yaml:"username"`
	Groups   []string `yaml:"groups"`
	UserARN  string   `yaml:"userarn,omitempty"`
	RoleARN  string   `yaml:"rolearn,omitempty"`
}

// EKSNodePool represent a node pool configuration
type EKSNodePool struct {
	Name                    string              `yaml:"name"`
	Version                 string              `yaml:"version"`
	MinSize                 int                 `yaml:"minSize"`
	MaxSize                 int                 `yaml:"maxSize"`
	InstanceType            string              `yaml:"instanceType"`
	OS                      string              `yaml:"os"`
	TargetGroups            []string            `yaml:"targetGroups"`
	MaxPods                 int                 `yaml:"maxPods"`
	SpotInstance            bool                `yaml:"spotInstance"`
	VolumeSize              int                 `yaml:"volumeSize"`
	Labels                  map[string]string   `yaml:"labels"`
	Taints                  []string            `yaml:"taints"`
	SubNetworks             []string            `yaml:"subnetworks"`
	Tags                    map[string]string   `yaml:"tags"`
	AdditionalFirewallRules []EKSNodePoolFwRule `yaml:"additionalFirewallRules"`
}

// EKSNodePoolFwRule represent an additional firewall rule to add to a specific node pool in the cluster
type EKSNodePoolFwRule struct {
	Name      string            `yaml:"name"`
	Direction string            `yaml:"direction"`
	CIDRBlock string            `yaml:"cidrBlock"`
	Protocol  string            `yaml:"protocol"`
	Ports     string            `yaml:"ports"`
	Tags      map[string]string `yaml:"tags"`
}
