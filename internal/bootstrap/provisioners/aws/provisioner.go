// Copyright (c) 2022 SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package aws

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/gobuffalo/packr/v2"
	"github.com/hashicorp/terraform-exec/tfexec"
	cfg "github.com/sighupio/furyctl/internal/bootstrap/configuration"
	"github.com/sighupio/furyctl/internal/configuration"

	log "github.com/sirupsen/logrus"
)

// InitMessage return a custom provisioner message the user will see once the cluster is ready to be updated
func (d *AWS) InitMessage() string {
	return `[AWS] - VPC and VPN

This provisioner creates a battle-tested AWS VPC with all the requirements
set to run a production-grade EKS cluster.

If you opt to create a private cluster, the provisioner will create a VPN Server that will
allow you to access the cluster from your local machine.
You can then use furyagent to manage credentials and access to the private network.
`
}

// UpdateMessage return a custom provisioner message the user will see once the cluster is updated
func (d *AWS) UpdateMessage() string {
	var output map[string]tfexec.OutputMeta
	output, err := d.terraform.Output(context.Background())
	if err != nil {
		log.Error("Can not get output values")
	}
	spec := d.config.Spec.(cfg.AWS)
	sshUsers := spec.VPN.SSHUsers
	var vpnOperatorName, vpcID string
	var vpnInstanceIPs, publicSubnetsIDs, privateSubnetsIDs []string
	err = json.Unmarshal(output["vpn_ip"].Value, &vpnInstanceIPs)
	if err != nil {
		log.Error("Can not get `vpn_ip` value")
	}
	err = json.Unmarshal(output["vpn_operator_name"].Value, &vpnOperatorName)
	if err != nil {
		log.Error("Can not get `vpn_operator_name` value")
	}
	err = json.Unmarshal(output["vpc_id"].Value, &vpcID)
	if err != nil {
		log.Error("Can not get `vpc_id` value")
	}
	err = json.Unmarshal(output["public_subnets"].Value, &publicSubnetsIDs)
	if err != nil {
		log.Error("Can not get `public_subnets` value")
	}
	err = json.Unmarshal(output["private_subnets"].Value, &privateSubnetsIDs)
	if err != nil {
		log.Error("Can not get `private_subnets` value")
	}

	vpnFragment := ""
	if len(vpnInstanceIPs) > 0 {
		vpnSSHFragment := ""
		for _, server := range vpnInstanceIPs {
			vpnSSHFragment = vpnSSHFragment + fmt.Sprintf("$ ssh %v@%v\n", vpnOperatorName, server)
		}
		vpnFragment = fmt.Sprintf(`
Your VPN instance IPs are: %v
Use the ssh %v username to access the VPN instance with any SSH key configured
for the following GitHub users: %v.

%v`, vpnInstanceIPs, vpnOperatorName, sshUsers, vpnSSHFragment)
	}

	return fmt.Sprintf(`[AWS] - VPC and VPN

All the bootstrap components are up to date.

VPC and VPN ready:

VPC: %v
Public Subnets: %v
Private Subnets: %v
%v
Then create a openvpn configuration (ovpn) file using the furyagent cli:

$ furyagent configure openvpn-client --client-name <your-name-goes-here> --config %v/secrets/furyagent.yml > <your-name-goes-here>.ovpn

Discover already registered vpn clients running:

$ furyagent configure openvpn-client --list --config %v/secrets/furyagent.yml

IMPORTANT! Connect to the VPN with the created ovpn profile to continue deploying
an AWS Kubernetes cluster.
`, vpcID, publicSubnetsIDs, privateSubnetsIDs, vpnFragment, d.terraform.WorkingDir(), d.terraform.WorkingDir())
}

// DestroyMessage return a custom provisioner message the user will see once the cluster is destroyed
func (d *AWS) DestroyMessage() string {
	return `[AWS] - VPC and VPN
All bootstrap components were destroyed.
VPN and VPC went away.

Had problems, contact us at sales@sighup.io.
`
}

// Enterprise return a boolean indicating it is an enterprise provisioner
func (d *AWS) Enterprise() bool {
	return false
}

// AWS represents a dummy provisioner
type AWS struct {
	terraform *tfexec.Terraform
	box       *packr.Box
	config    *configuration.Configuration
}

const (
	projectPath = "../../../../data/provisioners/bootstrap/aws"
)

func (d AWS) createVarFile() (err error) {
	var buffer bytes.Buffer
	spec := d.config.Spec.(cfg.AWS)

	buffer.WriteString(fmt.Sprintf("name = \"%v\"\n", d.config.Metadata.Name))
	buffer.WriteString(fmt.Sprintf("network_cidr = \"%v\"\n", spec.NetworkCIDR))
	buffer.WriteString(fmt.Sprintf("vpc_enabled = %v\n", spec.VPC.Enabled))
	buffer.WriteString(fmt.Sprintf("vpn_enabled = %v\n", spec.VPN.Enabled))

	if spec.VPC.Enabled {
		pubCIDRs := spec.VPCPublicSubnetsCIDRs
		if len(spec.VPC.PublicSubnetsCIDRs) > 0 {
			pubCIDRs = spec.VPC.PublicSubnetsCIDRs
		}

		priCIDRs := spec.VPCPrivateSubnetsCIDRs
		if len(spec.VPC.PrivateSubnetsCIDRs) > 0 {
			priCIDRs = spec.VPC.PrivateSubnetsCIDRs
		}

		buffer.WriteString(fmt.Sprintf("vpc_public_subnetwork_cidrs = [\"%v\"]\n", strings.Join(pubCIDRs, "\",\"")))
		buffer.WriteString(fmt.Sprintf("vpc_private_subnetwork_cidrs = [\"%v\"]\n", strings.Join(priCIDRs, "\",\"")))
	}

	if spec.VPN.Enabled {
		buffer.WriteString(fmt.Sprintf("vpn_vpc_id = \"%v\"\n", spec.VPN.VpcID))
		buffer.WriteString(fmt.Sprintf("vpn_public_subnets = [\"%v\"]\n", strings.Join(spec.VPN.PublicSubnets, "\",\"")))

		if spec.VPN.SubnetCIDR != "" {
			buffer.WriteString(fmt.Sprintf("vpn_subnetwork_cidr = \"%v\"\n", spec.VPN.SubnetCIDR))
		}

		buffer.WriteString(fmt.Sprintf("vpn_instances = %v\n", spec.VPN.Instances))
		if spec.VPN.Port != 0 {
			buffer.WriteString(fmt.Sprintf("vpn_port = %v\n", spec.VPN.Port))
		}
		if spec.VPN.InstanceType != "" {
			buffer.WriteString(fmt.Sprintf("vpn_instance_type = \"%v\"\n", spec.VPN.InstanceType))
		}
		if spec.VPN.DiskSize != 0 {
			buffer.WriteString(fmt.Sprintf("vpn_instance_disk_size = %v\n", spec.VPN.DiskSize))
		}
		if spec.VPN.OperatorName != "" {
			buffer.WriteString(fmt.Sprintf("vpn_operator_name = \"%v\"\n", spec.VPN.OperatorName))
		}
		if spec.VPN.DHParamsBits != 0 {
			buffer.WriteString(fmt.Sprintf("vpn_dhparams_bits = %v\n", spec.VPN.DHParamsBits))
		}
		if len(spec.VPN.OperatorCIDRs) != 0 {
			buffer.WriteString(fmt.Sprintf("vpn_operator_cidrs = [\"%v\"]\n", strings.Join(spec.VPN.OperatorCIDRs, "\",\"")))
		}
		if len(spec.VPN.SSHUsers) != 0 {
			buffer.WriteString(fmt.Sprintf("vpn_ssh_users = [\"%v\"]\n", strings.Join(spec.VPN.SSHUsers, "\",\"")))
		}
	}

	if len(spec.Tags) > 0 {
		var tags []byte
		tags, err = json.Marshal(spec.Tags)
		if err != nil {
			return err
		}
		buffer.WriteString(fmt.Sprintf("tags = %v\n", string(tags)))
	}

	tfVarsPath := fmt.Sprintf("%v/aws.tfvars", d.terraform.WorkingDir())

	if err := ioutil.WriteFile(tfVarsPath, buffer.Bytes(), 0600); err != nil {
		return err
	}

	if err := d.terraform.FormatWrite(context.Background(), tfexec.Dir(tfVarsPath)); err != nil {
		return err
	}

	return nil
}

// New instantiates a new AWS provisioner
func New(config *configuration.Configuration) *AWS {
	b := packr.New("AWS", projectPath)
	return &AWS{
		box:    b,
		config: config,
	}
}

// SetTerraformExecutor adds the terraform executor to this provisioner
func (d *AWS) SetTerraformExecutor(tf *tfexec.Terraform) {
	d.terraform = tf
}

// TerraformExecutor returns the current terraform executor of this provisioner
func (d *AWS) TerraformExecutor() (tf *tfexec.Terraform) {
	return d.terraform
}

// Box returns the box that has the files as binary data
func (d AWS) Box() *packr.Box {
	return d.box
}

// TerraformFiles returns the list of files conforming the terraform project
func (d AWS) TerraformFiles() []string {
	// TODO understand if it is possible to deduce these values somehow
	// find . -type f -follow -print
	return []string{
		"output.tf",
		"main.tf",
		"variables.tf",
	}
}

// Plan runs a dry run execution
func (d AWS) Prepare() (err error) {
	return nil
}

// Plan runs a dry run execution
func (d AWS) Plan() (err error) {
	log.Info("[DRYRUN] Updating AWS Bootstrap project")
	err = d.createVarFile()
	if err != nil {
		return err
	}
	changes, err := d.terraform.Plan(context.Background(), tfexec.VarFile(fmt.Sprintf("%v/aws.tfvars", d.terraform.WorkingDir())))
	if err != nil {
		log.Fatalf("[DRYRUN] Something went wrong while updating aws. %v", err)
		return err
	}
	if changes {
		log.Warn("[DRYRUN] Something changed along the time. Remove dryrun option to apply the desired state")
	} else {
		log.Info("[DRYRUN] Everything is up to date")
	}

	log.Info("[DRYRUN] AWS Updated")
	return nil
}

// Update runs terraform apply in the project
func (d AWS) Update() (string, error) {
	log.Info("Updating AWS Bootstrap project")
	err := d.createVarFile()
	if err != nil {
		return "", err
	}

	err = d.terraform.Apply(context.Background(), tfexec.VarFile(fmt.Sprintf("%v/aws.tfvars", d.terraform.WorkingDir())))
	if err != nil {
		log.Fatalf("Something went wrong while updating aws. %v", err)
		return "", err
	}

	log.Info("AWS Updated")
	return "", nil
}

// Destroy runs terraform destroy in the project
func (d AWS) Destroy() (err error) {
	log.Info("Destroying AWS Bootstrap project")
	err = d.createVarFile()
	if err != nil {
		return err
	}

	err = d.terraform.Destroy(context.Background(), tfexec.VarFile(fmt.Sprintf("%v/aws.tfvars", d.terraform.WorkingDir())))
	if err != nil {
		log.Fatalf("Something went wrong while destroying AWS Bootstrap project. %v", err)
		return err
	}
	log.Info("AWS Bootstrap destroyed")
	return nil
}
