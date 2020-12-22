// Copyright (c) 2020 SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package aws

import (
	"context"
	"encoding/json"
	"fmt"
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
set to run a production-grade private EKS cluster.

It creates a VPN server enables deploying the cluster from this computer
once connected to the VPN server.

Then, use furyagent to manage VPN profiles.
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
	var vpnInstanceIP, vpnOperatorName, vpcID string
	var publicSubnetsIDs, privateSubnetsIDs []string
	err = json.Unmarshal(output["vpn_ip"].Value, &vpnInstanceIP)
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

	return fmt.Sprintf(`[AWS] - VPC and VPN

All the bootstrap components are up to date.

VPC and VPN ready:

VPC: %v
Public Subnets: %v
Private Subnets: %v

Your VPN instance IP is: %v
Use the ssh %v username to access the VPN instance with any SSH key configured
for the following GitHub users: %v.

$ ssh %v@%v

Then create a openvpn configuration (ovpn) file using the furyagent cli:

$ furyagent configure openvpn-client --client-name <your-name-goes-here> --config %v/furyagent.yml > <your-name-goes-here>.ovpn

Discover already registered vpn clients running:

$ furyagent configure openvpn-client --list --config %v/furyagent.yml

IMPORTANT! Connect to the VPN with the created ovpn profile to continue deploying
an AWS Kubernetes cluster.
`, vpcID, publicSubnetsIDs, privateSubnetsIDs, vpnInstanceIP, vpnOperatorName, sshUsers, vpnOperatorName, vpnInstanceIP, d.terraform.WorkingDir(), d.terraform.WorkingDir())
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
	return true
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
func (d AWS) Plan() (err error) {
	log.Info("[DRYRUN] Updating AWS Bootstrap project")
	spec := d.config.Spec.(cfg.AWS)

	var opts []tfexec.PlanOption
	opts = append(opts, tfexec.Var(fmt.Sprintf("name=%v", d.config.Metadata.Name)))
	opts = append(opts, tfexec.Var(fmt.Sprintf("network_cidr=%v", spec.NetworkCIDR)))
	opts = append(opts, tfexec.Var(fmt.Sprintf("public_subnetwork_cidrs=[\"%v\"]", strings.Join(spec.PublicSubnetsCIDRs, "\",\""))))
	opts = append(opts, tfexec.Var(fmt.Sprintf("private_subnetwork_cidrs=[\"%v\"]", strings.Join(spec.PrivateSubnetsCIDRs, "\",\""))))
	opts = append(opts, tfexec.Var(fmt.Sprintf("vpn_subnetwork_cidr=%v", spec.VPN.SubnetCIDR)))
	if len(spec.Tags) > 0 {
		var tags []byte
		tags, err = json.Marshal(spec.Tags)
		if err != nil {
			return err
		}
		opts = append(opts, tfexec.Var(fmt.Sprintf("tags=%v\n", string(tags))))
	}
	if spec.VPN.Port != 0 {
		opts = append(opts, tfexec.Var(fmt.Sprintf("vpn_port=%v", spec.VPN.Port)))
	}
	if spec.VPN.InstanceType != "" {
		opts = append(opts, tfexec.Var(fmt.Sprintf("vpn_instance_type=%v", spec.VPN.InstanceType)))
	}
	if spec.VPN.DiskSize != 0 {
		opts = append(opts, tfexec.Var(fmt.Sprintf("vpn_instance_disk_size=%v", spec.VPN.DiskSize)))
	}
	if spec.VPN.OperatorName != "" {
		opts = append(opts, tfexec.Var(fmt.Sprintf("vpn_operator_name=%v", spec.VPN.OperatorName)))
	}
	if spec.VPN.DHParamsBits != 0 {
		opts = append(opts, tfexec.Var(fmt.Sprintf("vpn_dhparams_bits=%v", spec.VPN.DHParamsBits)))
	}
	if len(spec.VPN.OperatorCIDRs) != 0 {
		opts = append(opts, tfexec.Var(fmt.Sprintf("vpn_operator_cidrs=[\"%v\"]", strings.Join(spec.VPN.OperatorCIDRs, "\",\""))))
	}
	if len(spec.VPN.SSHUsers) != 0 {
		opts = append(opts, tfexec.Var(fmt.Sprintf("vpn_ssh_users=[\"%v\"]", strings.Join(spec.VPN.SSHUsers, "\",\""))))
	}
	changes, err := d.terraform.Plan(context.Background(), opts...)
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
func (d AWS) Update() (err error) {
	log.Info("Updating AWS Bootstrap project")
	spec := d.config.Spec.(cfg.AWS)

	var opts []tfexec.ApplyOption
	opts = append(opts, tfexec.Var(fmt.Sprintf("name=%v", d.config.Metadata.Name)))
	opts = append(opts, tfexec.Var(fmt.Sprintf("network_cidr=%v", spec.NetworkCIDR)))
	opts = append(opts, tfexec.Var(fmt.Sprintf("public_subnetwork_cidrs=[\"%v\"]", strings.Join(spec.PublicSubnetsCIDRs, "\",\""))))
	opts = append(opts, tfexec.Var(fmt.Sprintf("private_subnetwork_cidrs=[\"%v\"]", strings.Join(spec.PrivateSubnetsCIDRs, "\",\""))))
	opts = append(opts, tfexec.Var(fmt.Sprintf("vpn_subnetwork_cidr=%v", spec.VPN.SubnetCIDR)))
	if len(spec.Tags) > 0 {
		var tags []byte
		tags, err = json.Marshal(spec.Tags)
		if err != nil {
			return err
		}
		opts = append(opts, tfexec.Var(fmt.Sprintf("tags=%v\n", string(tags))))
	}
	if spec.VPN.Port != 0 {
		opts = append(opts, tfexec.Var(fmt.Sprintf("vpn_port=%v", spec.VPN.Port)))
	}
	if spec.VPN.InstanceType != "" {
		opts = append(opts, tfexec.Var(fmt.Sprintf("vpn_instance_type=%v", spec.VPN.InstanceType)))
	}
	if spec.VPN.DiskSize != 0 {
		opts = append(opts, tfexec.Var(fmt.Sprintf("vpn_instance_disk_size=%v", spec.VPN.DiskSize)))
	}
	if spec.VPN.OperatorName != "" {
		opts = append(opts, tfexec.Var(fmt.Sprintf("vpn_operator_name=%v", spec.VPN.OperatorName)))
	}
	if spec.VPN.DHParamsBits != 0 {
		opts = append(opts, tfexec.Var(fmt.Sprintf("vpn_dhparams_bits=%v", spec.VPN.DHParamsBits)))
	}
	if len(spec.VPN.OperatorCIDRs) != 0 {
		opts = append(opts, tfexec.Var(fmt.Sprintf("vpn_operator_cidrs=[\"%v\"]", strings.Join(spec.VPN.OperatorCIDRs, "\",\""))))
	}
	if len(spec.VPN.SSHUsers) != 0 {
		opts = append(opts, tfexec.Var(fmt.Sprintf("vpn_ssh_users=[\"%v\"]", strings.Join(spec.VPN.SSHUsers, "\",\""))))
	}
	err = d.terraform.Apply(context.Background(), opts...)
	if err != nil {
		log.Fatalf("Something went wrong while updating aws. %v", err)
		return err
	}

	log.Info("AWS Updated")
	return nil
}

// Destroy runs terraform destroy in the project
func (d AWS) Destroy() (err error) {
	log.Info("Destroying AWS Bootstrap project")
	spec := d.config.Spec.(cfg.AWS)

	var opts []tfexec.DestroyOption
	opts = append(opts, tfexec.Var(fmt.Sprintf("name=%v", d.config.Metadata.Name)))
	opts = append(opts, tfexec.Var(fmt.Sprintf("network_cidr=%v", spec.NetworkCIDR)))
	opts = append(opts, tfexec.Var(fmt.Sprintf("public_subnetwork_cidrs=[\"%v\"]", strings.Join(spec.PublicSubnetsCIDRs, "\",\""))))
	opts = append(opts, tfexec.Var(fmt.Sprintf("private_subnetwork_cidrs=[\"%v\"]", strings.Join(spec.PrivateSubnetsCIDRs, "\",\""))))
	opts = append(opts, tfexec.Var(fmt.Sprintf("vpn_subnetwork_cidr=%v", spec.VPN.SubnetCIDR)))
	if len(spec.Tags) > 0 {
		var tags []byte
		tags, err = json.Marshal(spec.Tags)
		if err != nil {
			return err
		}
		opts = append(opts, tfexec.Var(fmt.Sprintf("tags=%v\n", string(tags))))
	}
	if spec.VPN.Port != 0 {
		opts = append(opts, tfexec.Var(fmt.Sprintf("vpn_port=%v", spec.VPN.Port)))
	}
	if spec.VPN.InstanceType != "" {
		opts = append(opts, tfexec.Var(fmt.Sprintf("vpn_instance_type=%v", spec.VPN.InstanceType)))
	}
	if spec.VPN.DiskSize != 0 {
		opts = append(opts, tfexec.Var(fmt.Sprintf("vpn_instance_disk_size=%v", spec.VPN.DiskSize)))
	}
	if spec.VPN.OperatorName != "" {
		opts = append(opts, tfexec.Var(fmt.Sprintf("vpn_operator_name=%v", spec.VPN.OperatorName)))
	}
	if spec.VPN.DHParamsBits != 0 {
		opts = append(opts, tfexec.Var(fmt.Sprintf("vpn_dhparams_bits=%v", spec.VPN.DHParamsBits)))
	}
	if len(spec.VPN.OperatorCIDRs) != 0 {
		opts = append(opts, tfexec.Var(fmt.Sprintf("vpn_operator_cidrs=[\"%v\"]", strings.Join(spec.VPN.OperatorCIDRs, "\",\""))))
	}
	if len(spec.VPN.SSHUsers) != 0 {
		opts = append(opts, tfexec.Var(fmt.Sprintf("vpn_ssh_users=[\"%v\"]", strings.Join(spec.VPN.SSHUsers, "\",\""))))
	}
	err = d.terraform.Destroy(context.Background(), opts...)
	if err != nil {
		log.Fatalf("Something went wrong while destroying AWS Bootstrap project. %v", err)
		return err
	}
	log.Info("AWS Bootstrap destroyed")
	return nil
}
