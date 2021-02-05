// Copyright (c) 2020 SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package configuration

// GCP represents the configuration spec of a AWS bootstrap project including VPC and VPN
type GCP struct {
	PublicSubnetsCIDRs  []string          `yaml:"publicSubnetsCIDRs"`
	PrivateSubnetsCIDRs []string          `yaml:"privateSubnetsCIDRs"`
	ClusterNetwork      GCPClusterNetwork `yaml:"clusterNetwork"`
	VPN                 GCPVPN            `yaml:"vpn"`
	Tags                map[string]string `yaml:"tags"`
}

// GCPClusterNetwork represents the cluster network configuration
type GCPClusterNetwork struct {
	SubnetworkCIDR        string `yaml:"subnetworkCIDR"`
	ControlPlaneCIDR      string `yaml:"controlPlaneCIDR"`
	PodSubnetworkCIDR     string `yaml:"podSubnetworkCIDR"`
	ServiceSubnetworkCIDR string `yaml:"serviceSubnetworkCIDR"`
}

// GCPVPN represents an VPN configuration
type GCPVPN struct {
	Instances     int      `yaml:"instances"`
	Port          int      `yaml:"port"`
	InstanceType  string   `yaml:"instanceType"`
	DiskSize      int      `yaml:"diskSize"`
	OperatorName  string   `yaml:"operatorName"`
	DHParamsBits  int      `yaml:"dhParamsBits"`
	SubnetCIDR    string   `yaml:"subnetCIDR"`
	SSHUsers      []string `yaml:"sshUsers"`
	OperatorCIDRs []string `yaml:"operatorCIDRs"`
}
