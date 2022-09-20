// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package configuration TODO
package configuration

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	bootstrapcfg "github.com/sighupio/furyctl/internal/bootstrap/configuration"
	clustercfg "github.com/sighupio/furyctl/internal/cluster/configuration"
)

// Template generates a yaml with a sample configuration requested by the client
func Template(kind, provisioner string) (string, error) {
	config := Configuration{}
	config.Kind = kind
	config.Provisioner = provisioner
	switch {
	case kind == "Bootstrap":
		err := bootstrapTemplate(&config)
		if err != nil {
			return "", err
		}
	case kind == "Cluster":
		err := clusterTemplate(&config)
		if err != nil {
			return "", err
		}
	default:
		logrus.Errorf("Error creating a template configuration file. Parser not found for %v kind", kind)
		return "", fmt.Errorf("error creating a template configuration file. Parser not found for %v kind", kind)
	}
	b, err := yaml.Marshal(config)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func createBase(config *Configuration) {
	config.Metadata = Metadata{
		Name: "nameOfTheProject # Used to identify your resources",
		Labels: map[string]interface{}{
			"yourLabel": "yourValue # You Can add as much labels as you required. You can add bool, string, number values",
			"boolLabel": true,
		},
	}
	config.Executor = TerraformExecutor{
		StateConfiguration: StateConfiguration{
			Backend: "local # Specify your backend configuration. local is the default one. https://www.terraform.io/docs/configuration/backend.html",
			Config: map[string]string{
				"path": "workdir/terraform.state # Set the backend configuration parameter. this path attribute works with the local backend.",
			},
		},
	}
}

func bootstrapTemplate(config *Configuration) error {
	switch {
	case config.Provisioner == "aws":
		spec := bootstrapcfg.AWS{
			NetworkCIDR:         "10.0.0.0/16 # Required. Specific VPC Network CIDR to create",
			PublicSubnetsCIDRs:  []string{"Required", "10.0.10.0/24", "10.0.20.0/24", "10.0.30.0/24"},
			PrivateSubnetsCIDRs: []string{"Required", "10.0.192.0/24", "10.0.182.0/24", "10.0.172.0/24"},
			VPN: bootstrapcfg.AWSVPN{
				Instances:     2,
				Port:          1194,
				InstanceType:  "t3.micro # This is the default value. Specify any AWS EC2 instance type",
				DiskSize:      50,
				OperatorName:  "sighup # This is the default value. SSH User to access the instance",
				DHParamsBits:  2048,
				SubnetCIDR:    "172.16.0.0/16 # Required attribute. Should be different from the networkCIDR",
				SSHUsers:      []string{"Required", "angelbarrera92", "jnardiello"},
				OperatorCIDRs: []string{"0.0.0.0/0", "This is the default value"},
			},
			Tags: map[string]string{
				"myTag": "MyValue # Use this tags to annotate all resources. Optional",
			},
		}
		config.Spec = spec
	case config.Provisioner == "gcp":
		spec := bootstrapcfg.GCP{
			PublicSubnetsCIDRs:  []string{"Required", "10.0.1.0/24", "10.0.2.0/24"},
			PrivateSubnetsCIDRs: []string{"Required", "10.0.11.0/24", "10.0.12.0/24"},
			ClusterNetwork: bootstrapcfg.GCPClusterNetwork{
				ControlPlaneCIDR:      "10.0.0.0/28 # Optional. Control Plane CIDR. This value is the default one",
				SubnetworkCIDR:        "10.1.0.0/16 # Required. Cluster nodes subnetwork",
				PodSubnetworkCIDR:     "10.2.0.0/16 # Required. Pod subnetwork CIDR",
				ServiceSubnetworkCIDR: "10.3.0.0/16 # Required. Service subnetwork CIDR",
			},
			VPN: bootstrapcfg.GCPVPN{
				Instances:     2,
				Port:          1194,
				InstanceType:  "n1-standard-1 # This is the default value. Specify any GCP instance type",
				DiskSize:      50,
				OperatorName:  "sighup # This is the default value. SSH User to access the instance",
				DHParamsBits:  2048,
				SubnetCIDR:    "172.16.0.0/16 # Required attribute. Should be different from the networkCIDR",
				SSHUsers:      []string{"Required", "angelbarrera92", "jnardiello"},
				OperatorCIDRs: []string{"0.0.0.0/0", "This is the default value"},
			},
			Tags: map[string]string{
				"myTag": "MyValue # Use this tags to annotate all resources. Optional",
			},
		}
		config.Spec = spec
	default:
		logrus.Errorf("Error creating a template configuration file. Parser not found for %v provisioner", config.Provisioner)
		return fmt.Errorf("error creating a template configuration file. Parser not found for %v provisioner", config.Provisioner)
	}
	createBase(config)
	return nil
}

func clusterTemplate(config *Configuration) error {
	switch {
	case config.Provisioner == "eks":
		spec := clustercfg.EKS{
			Version:      "1.20 # EKS Control plane version",
			Network:      "vpc-id1 # Identificator of the VPC",
			SubNetworks:  []string{"subnet-id1 # Identificator of the subnets"},
			DMZCIDRRange: clustercfg.DMZCIDRRange{Values: []string{"10.0.0.0/8", "Required. Network CIDR range from where cluster control plane will be accessible"}},
			SSHPublicKey: "sha-rsa 190jd0132w. Required. Cluster administrator public ssh key. Used to access cluster nodes.",
			Tags: map[string]string{
				"myTag": "MyValue # Use this tags to annotate all resources. Optional",
			},
			Auth: clustercfg.EKSAuth{
				AdditionalAccounts: []string{"777777777777", "88888888888", "# Additional AWS account numbers to add to the aws-auth configmap"},
				Users: []clustercfg.EKSAuthData{
					{
						Username: "user1 # Any username",
						Groups:   []string{"system:masters", "# Any K8S Group"},
						UserARN:  "arn:user:7777777... # The user ARN",
					},
				},
				Roles: []clustercfg.EKSAuthData{
					{
						Username: "user1 # Any username",
						Groups:   []string{"system:masters", "# Any K8S Group"},
						RoleARN:  "arn:role:7777777... # The role ARN",
					},
				},
			},
			NodePools: []clustercfg.EKSNodePool{
				{
					Name:         "my-node-pool. Required. Name of the node pool",
					Version:      "1.23. Required. null to use Control Plane version.",
					MinSize:      0,
					MaxSize:      1,
					InstanceType: "t3.micro. Required. AWS EC2 isntance types",
					MaxPods:      110,
					VolumeSize:   50,
					SubNetworks:  []string{"subnet-1", "# any subnet id where you want your nodes running"},
					Labels: map[string]string{
						"environment": "dev. # Node labels. Use it to tag nodes then use it on Kubernetes",
					},
					OS:           "ami-12345 # The ami-id to use. Do not specify to use the default one",
					TargetGroups: []string{"target-12345", "the target-id to associate the instance to a loadbalancer"},
					Taints:       []string{"key1=value1:NoSchedule. As an example"},
					Tags: map[string]string{
						"myTag": "MyValue # Use this tags to annotate nodepool resources. Optional",
					},
					AdditionalFirewallRules: []clustercfg.EKSNodePoolFwRule{
						{
							Name:      "The name of rule # Identify the rule using a description",
							Direction: "ingress|egress # Choose one",
							CIDRBlock: "0.0.0.0/0 # CIDR Block",
							Protocol:  "TCP|UDP # Any supported AWS security group protocol",
							Ports:     "80-80 # Port range. This one means from 80 to 80",
							Tags: map[string]string{
								"myTag": "MyValue # Use this tags to annotate nodepool resources. Optional",
							},
						},
					},
				},
			},
		}
		config.Spec = spec
	case config.Provisioner == "gke":
		spec := clustercfg.GKE{
			Version:                        "1.23.12-gke.1206 # GKE Control plane version",
			Network:                        "vpc-id1 # Identificator of the Network",
			NetworkProjectID:               "12309123 # OPTIONAL. The project ID of the shared VPC's host (for shared vpc support)",
			ControlPlaneCIDR:               "10.0.0.0/28 # OPTIONAL. DEFAULT VALUE. The IP range in CIDR notation to use for the hosted master network",
			AdditionalFirewallRules:        true,
			AdditionalClusterFirewallRules: false,
			DisableDefaultSNAT:             false,
			SubNetworks: []string{
				"subnet-id0 # Identificator of the subnets. Index 0: Cluster Subnet",
				"subnet-id1 # Identificator of the subnets. Index 1: Pod Subnet",
				"subnet-id2 # Identificator of the subnets. Index 1: Service Subnet",
			},

			DMZCIDRRange: clustercfg.DMZCIDRRange{Values: []string{"10.0.0.0/8", "Required. Network CIDR range from where cluster control plane will be accessible"}},
			SSHPublicKey: "sha-rsa 190jd0132w. Required. Cluster administrator public ssh key. Used to access cluster nodes.",
			Tags: map[string]string{
				"myTag": "MyValue # Use this tags to annotate all resources. Optional",
			},

			NodePools: []clustercfg.GKENodePool{
				{
					Name:         "my-node-pool. Required. Name of the node pool",
					Version:      "1.23.12-gke.1206. Required. null to use Control Plane version.",
					MinSize:      1,
					MaxSize:      1,
					InstanceType: "n1-standard-1. Required. GCP instance types",
					OS:           "COS # The operating system to use. Do not specify to use the default one (COS)",
					MaxPods:      110,
					VolumeSize:   50,
					SubNetworks:  []string{"subnet-1", "# availability zones (example: us-central1-a) where to place the nodes. Useful to don't create them on all zones"},
					Labels: map[string]string{
						"environment": "dev. # Node labels. Use it to tag nodes then use it on Kubernetes",
					},
					Taints: []string{"key1=value1:NoSchedule. As an example"},
					Tags: map[string]string{
						"myTag": "MyValue # Use this tags to annotate nodepool resources. Optional",
					},
					AdditionalFirewallRules: []clustercfg.GKENodePoolFwRule{
						{
							Name:      "The name of rule # Identify the rule using a description",
							Direction: "ingress|egress # Choose one",
							CIDRBlock: "0.0.0.0/0 # CIDR Block",
							Protocol:  "TCP|UDP # Any supported GCP security group protocol",
							Ports:     "80-80 # Port range. This one means from 80 to 80",
							Tags: map[string]string{
								"myTag": "MyValue # Use this tags to annotate nodepool resources. Optional",
							},
						},
					},
				},
			},
		}
		config.Spec = spec
	case config.Provisioner == "vsphere":
		spec := clustercfg.VSphere{
			Version:              "1.20.5 # Place here the Kubernetes version you want to use",
			ControlPlaneEndpoint: "my-cluster.localdomain # OPTIONAL. Kubernetes control plane endpoint. Default to the VIP of the load balancer",
			ETCDConfig: clustercfg.VSphereETCDConfig{
				Version: "v3.4.15 # OPTIONAL. Place there the ETCD version you want to use",
			},
			OIDCConfig: clustercfg.VSphereOIDCConfig{
				IssuerURL: "https://dex.internal.example.com/ # OPTIONAL. Place here the issuer URL of your oidc provider",
				ClientID:  "oidc-auth-client # OPTIONAL. Place here the client ID",
				CAFile:    "/etc/pki/ca-trust/source/anchors/example.com.cer # OPTIONAL. The CA certificate to use",
			},
			CRIConfig: clustercfg.VSphereCRIConfig{
				Version: "18.06.2.ce # OPTIONAL. This is the default value for oracle linux docker CRI",
				DNS:     []string{"1.1.1.1", "8.8.8.8", "# OPTIONAL. Set here your DNS servers"},
				Proxy:   "\"HTTP_PROXY=http://systems.example.com:8080\" \"NO_PROXY=.example.com,.group.example.com\"",
				Mirrors: []string{"https://mirror.gcr.io", "# OPTIONAL. Set here your dockerhub mirrors"},
			},
			EnvironmentName: "production # The environment name of the cluster",
			Config: clustercfg.VSphereConfig{
				DatacenterName: "westeros # Get the name of datacenter from vShpere dashboard",
				Datastore:      "main # Get the name of datastore from vSphere dashboard",
				EsxiHost:       []string{"host1", "host2", "host3", "# Names of the hosts where the VMs are going to be created. Use only when not specifying a vSphere cluster"},
				Cluster:        "cluster1 # (Optional) vSphere Cluster (resource pool) where the Virtual Machines will be created. Use this option or specify the EsxiHost list when you don't have a cluster.",
			},
			NetworkConfig: clustercfg.VSphereNetworkConfig{
				Name:        "main-network # The name of the vSphere network",
				Gateway:     "10.0.0.1 # The IP of the network gateway",
				Nameservers: []string{"8.8.4.4", "1.1.1.1"},
				Domain:      "localdomain",
				IPOffset:    0,
			},
			Boundary: true,
			LoadBalancerNode: clustercfg.VSphereKubeLoadBalancer{
				Count:            1,
				Template:         "ubuntu-20.04 # The name of the base image to use for the VMs",
				CustomScriptPath: "./do-something.sh # A script that you want to run after first boot",
			},
			MasterNode: clustercfg.VSphereKubeNode{
				Count:    1,
				CPU:      1,
				MemSize:  4096,
				DiskSize: 100,
				Template: "ubuntu-20.04 # The name of the base image to use for the VMs",
				Labels: map[string]string{
					"environment": "dev. # Node labels. Use it to tag nodes then use it on Kubernetes",
				},
				Taints: []string{"key1=value1:NoSchedule. As an example"},
			},
			InfraNode: clustercfg.VSphereKubeNode{
				Count:    1,
				CPU:      1,
				MemSize:  8192,
				DiskSize: 100,
				Template: "ubuntu-20.04 # The name of the base image to use for the VMs",
				Labels: map[string]string{
					"environment": "dev. # Node labels. Use it to tag nodes then use it on Kubernetes",
				},
				Taints: []string{"key1=value1:NoSchedule. As an example"},
			},
			NodePools: []clustercfg.VSphereKubeNode{
				{
					Role:     "applications",
					Count:    1,
					CPU:      1,
					MemSize:  8192,
					DiskSize: 100,
					Template: "ubuntu-20.04 # The name of the base image to use for the VMs",
					Labels: map[string]string{
						"environment": "dev. # Node labels. Use it to tag nodes then use it on Kubernetes",
					},
					Taints: []string{"key1=value1:NoSchedule. As an example"},
				},
			},
			ClusterPODCIDR: "172.21.0.0/16",
			ClusterSVCCIDR: "172.23.0.0/16",
			ClusterCIDR:    "10.2.0.0/16",
			SSHPublicKey: []string{
				"/home/foo/.ssh/id_rsa.pub",
			},
		}
		config.Spec = spec
	default:
		logrus.Errorf("error creating a template configuration file. Parser not found for %v provisioner", config.Provisioner)
		return fmt.Errorf("error creating a template configuration file. Parser not found for %v provisioner", config.Provisioner)
	}
	createBase(config)
	return nil
}
