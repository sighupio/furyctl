// Copyright (c) 2020 SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package configuration

import (
	"fmt"

	bootstrapcfg "github.com/sighupio/furyctl/internal/bootstrap/configuration"
	clustercfg "github.com/sighupio/furyctl/internal/cluster/configuration"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

// Template generates a yaml with a sample configuration requested by the client
func Template(kind string, provisioner string) (string, error) {
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
		log.Errorf("Error creating a template configuration file. Parser not found for %v kind", kind)
		return "", fmt.Errorf("Error creating a template configuration file. Parser not found for %v kind", kind)
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
		Path:    "/your/terraform/binary # Set this value if you need to use an already installed terraform binary",
		Version: "0.12.29 # Set this value to download a specific terraform version. If version and path is not specified, latest terraform version will be downloaded",
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
		log.Errorf("Error creating a template configuration file. Parser not found for %v provisioner", config.Provisioner)
		return fmt.Errorf("Error creating a template configuration file. Parser not found for %v provisioner", config.Provisioner)
	}
	createBase(config)
	return nil
}

func clusterTemplate(config *Configuration) error {
	switch {
	case config.Provisioner == "eks":
		spec := clustercfg.EKS{
			Version:      "1.18 # EKS Control plane version",
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
					Version:      "1.18. Required. null to use Control Plane version.",
					MinSize:      0,
					MaxSize:      1,
					InstanceType: "t3.micro. Required. AWS EC2 isntance types",
					MaxPods:      110,
					VolumeSize:   50,
					SubNetworks:  []string{"subnet-1", "# any subnet id where you want your nodes running"},
					Labels: map[string]string{
						"environment": "dev. # Node labels. Use it to tag nodes then use it on Kubernetes",
					},
					Taints: []string{"key1=value1:NoSchedule. As an example"},
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
	default:
		log.Errorf("Error creating a template configuration file. Parser not found for %v provisioner", config.Provisioner)
		return fmt.Errorf("Error creating a template configuration file. Parser not found for %v provisioner", config.Provisioner)
	}
	createBase(config)
	return nil
}
