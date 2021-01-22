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
			PublicSubnetsCIDRs:  []string{"Required", "10.0.10.0/24", "10.0.10.0/24", "10.0.10.0/24"},
			PrivateSubnetsCIDRs: []string{"Required", "10.0.192.0/24", "10.0.192.0/24", "10.0.192.0/24"},
			VPN: bootstrapcfg.AWSVPN{
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
			DMZCIDRRange: "10.0.0.0/8. Required. Network CIDR range from where cluster control plane will be accessible",
			SSHPublicKey: "sha-rsa 190jd0132w. Required. Cluster administrator public ssh key. Used to access cluster nodes.",
			Tags: map[string]string{
				"myTag": "MyValue # Use this tags to annotate all resources. Optional",
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
					Labels: map[string]string{
						"environment": "dev. # Node labels. Use it to tag nodes then use it on Kubernetes",
					},
					Taints: []string{"key1=value1:NoSchedule. As an example"},
					Tags: map[string]string{
						"myTag": "MyValue # Use this tags to annotate nodepool resources. Optional",
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
