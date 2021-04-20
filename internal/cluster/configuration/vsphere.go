// Copyright (c) 2020 SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package configuration TODO
package configuration

// VSphere represents the configuration spec of a VSphere Cluster
type VSphere struct {
	Version         string        `yaml:"version"`
	EnvironmentName string        `yaml:"environmentName"`
	Config          VSphereConfig `yaml:"config"`

	NetworkConfig VSphereNetworkConfig `yaml:"networkConfig"`

	Boundary bool `yaml:"boundary"`

	LoadBalancerNode VSphereKubeLoadBalancer `yaml:"lbNode"`
	MasterNode       VSphereKubeNode         `yaml:"masterNode"`
	InfraNode        VSphereKubeNode         `yaml:"infraNode"`
	NodePools        []VSphereKubeNode       `yaml:"nodePools"`

	ClusterPODCIDR string   `yaml:"clusterPODCIDR"`
	ClusterSVCCIDR string   `yaml:"clusterSVCCIDR"`
	ClusterCIDR    string   `yaml:"clusterCIDR"`
	SSHPublicKey   []string `yaml:"sshPublicKeys"`
}

type VSphereKubeLoadBalancer struct {
	Count            int    `yaml:"count"`
	Template         string `yaml:"template"`
	CustomScriptPath string `yaml:"customScriptPath"`
}

type VSphereKubeNode struct {
	Role     string            `yaml:"role"`
	Count    int               `yaml:"count"`
	CPU      int               `yaml:"cpu"`
	MemSize  int               `yaml:"memSize"`
	DiskSize int               `yaml:"diskSize"`
	Template string            `yaml:"template"`
	Labels   map[string]string `yaml:"labels"`
	Taints   []string          `yaml:"taints"`
	CustomScriptPath string `yaml:"customScriptPath"`
}

// TODO: can you do that?
// type VSphereKubeNodePool struct {
// 	role string `yaml:"role"`
// 	VSphereKubeNode
// }

type VSphereNetworkConfig struct {
	Name        string   `yaml:"name"`
	Gateway     string   `yaml:"gateway"`
	Nameservers []string `yaml:"nameservers"`
	Domain      string   `yaml:"domain"`
}

type VSphereConfig struct {
	DatacenterName string   `yaml:"datacenterName"`
	Datastore      string   `yaml:"datastore"`
	EsxiHost       []string `yaml:"esxiHosts"`
}
