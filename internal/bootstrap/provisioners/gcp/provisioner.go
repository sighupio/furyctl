// Copyright (c) 2022 SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gcp

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
func (d *GCP) InitMessage() string {
	return `[GCP] - VPC and VPN

This provisioner creates a battle-tested GCP VPC with all the requirements
set to run a production-grade private GKE cluster.

It creates VPN servers enables deploying the cluster from this computer
once connected to the VPN server.

Then, use furyagent to manage VPN profiles.
`
}

// UpdateMessage return a custom provisioner message the user will see once the cluster is updated
func (d *GCP) UpdateMessage() string {
	var output map[string]tfexec.OutputMeta
	output, err := d.terraform.Output(context.Background())
	if err != nil {
		log.Error("Can not get output values")
	}
	spec := d.config.Spec.(cfg.GCP)
	sshUsers := spec.VPN.SSHUsers
	var vpnOperatorName, networkName, clusterSubnet, podSubnet, serviceSubnet string
	var vpnInstanceIPs, publicSubnetsIDs, privateSubnetsIDs []string

	type subnets map[string]string

	var additionalClusterSubnet []subnets
	err = json.Unmarshal(output["vpn_ip"].Value, &vpnInstanceIPs)
	if err != nil {
		log.Error("Can not get `vpn_ip` value")
	}
	err = json.Unmarshal(output["vpn_operator_name"].Value, &vpnOperatorName)
	if err != nil {
		log.Error("Can not get `vpn_operator_name` value")
	}
	err = json.Unmarshal(output["network_name"].Value, &networkName)
	if err != nil {
		log.Error("Can not get `network_name` value")
	}
	err = json.Unmarshal(output["public_subnets"].Value, &publicSubnetsIDs)
	if err != nil {
		log.Error("Can not get `public_subnets` value")
	}
	err = json.Unmarshal(output["private_subnets"].Value, &privateSubnetsIDs)
	if err != nil {
		log.Error("Can not get `private_subnets` value")
	}
	err = json.Unmarshal(output["cluster_subnet"].Value, &clusterSubnet)
	if err != nil {
		log.Error("Can not get `cluster_subnet` value")
	}
	err = json.Unmarshal(output["additional_cluster_subnet"].Value, &additionalClusterSubnet)
	if err != nil {
		log.Error("Can not get `additional_cluster_subnet` value")
	}

	for _, subnet := range additionalClusterSubnet {
		if strings.Contains(subnet["name"], "pod-subnet") {
			podSubnet = subnet["name"]
		} else if strings.Contains(subnet["name"], "service-subnet") {
			serviceSubnet = subnet["name"]
		}
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

	return fmt.Sprintf(`[GCP] - VPC and VPN

All the bootstrap components are up to date.

VPC and VPN ready:

VPC: %v
Public Subnets	: %v
Private Subnets	: %v
Cluster Subnet	: %v
  Pod Subnet	: %v
  Service Subnet: %v
%v
Then create a openvpn configuration (ovpn) file using the furyagent cli:

$ furyagent configure openvpn-client --client-name <your-name-goes-here> --config %v/secrets/furyagent.yml > <your-name-goes-here>.ovpn

Discover already registered vpn clients running:

$ furyagent configure openvpn-client --list --config %v/secrets/furyagent.yml

IMPORTANT! Connect to the VPN with the created ovpn profile to continue deploying
an GKE Kubernetes cluster.
`, networkName, publicSubnetsIDs, privateSubnetsIDs, clusterSubnet, podSubnet, serviceSubnet, vpnFragment, d.terraform.WorkingDir(), d.terraform.WorkingDir())
}

// DestroyMessage return a custom provisioner message the user will see once the cluster is destroyed
func (d *GCP) DestroyMessage() string {
	return `[GCP] - VPC and VPN
All bootstrap components were destroyed.
VPN and VPC went away.

Had problems, contact us at sales@sighup.io.
`
}

// Enterprise return a boolean indicating it is an enterprise provisioner
func (d *GCP) Enterprise() bool {
	return false
}

// GCP represents a dummy provisioner
type GCP struct {
	terraform *tfexec.Terraform
	box       *packr.Box
	config    *configuration.Configuration
}

const (
	projectPath = "../../../../data/provisioners/bootstrap/gcp"
)

func (d GCP) createVarFile() (err error) {
	var buffer bytes.Buffer
	spec := d.config.Spec.(cfg.GCP)

	buffer.WriteString(fmt.Sprintf("name = \"%v\"\n", d.config.Metadata.Name))
	buffer.WriteString(fmt.Sprintf("public_subnetwork_cidrs = [\"%v\"]\n", strings.Join(spec.PublicSubnetsCIDRs, "\",\"")))
	buffer.WriteString(fmt.Sprintf("private_subnetwork_cidrs = [\"%v\"]\n", strings.Join(spec.PrivateSubnetsCIDRs, "\",\"")))

	if spec.ClusterNetwork.ControlPlaneCIDR != "" {
		buffer.WriteString(fmt.Sprintf("cluster_control_plane_cidr_block = \"%v\"\n", spec.ClusterNetwork.ControlPlaneCIDR))
	}
	buffer.WriteString(fmt.Sprintf("cluster_subnetwork_cidr = \"%v\"\n", spec.ClusterNetwork.SubnetworkCIDR))
	buffer.WriteString(fmt.Sprintf("cluster_pod_subnetwork_cidr = \"%v\"\n", spec.ClusterNetwork.PodSubnetworkCIDR))
	buffer.WriteString(fmt.Sprintf("cluster_service_subnetwork_cidr = \"%v\"\n", spec.ClusterNetwork.ServiceSubnetworkCIDR))

	buffer.WriteString(fmt.Sprintf("vpn_subnetwork_cidr = \"%v\"\n", spec.VPN.SubnetCIDR))
	if len(spec.Tags) > 0 {
		var tags []byte
		tags, err = json.Marshal(spec.Tags)
		if err != nil {
			return err
		}
		buffer.WriteString(fmt.Sprintf("tags = %v\n", string(tags)))
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

	err = ioutil.WriteFile(fmt.Sprintf("%v/gcp.tfvars", d.terraform.WorkingDir()), buffer.Bytes(), 0600)
	if err != nil {
		return err
	}
	err = d.terraform.FormatWrite(context.Background(), tfexec.Dir(fmt.Sprintf("%v/gcp.tfvars", d.terraform.WorkingDir())))
	if err != nil {
		return err
	}
	return nil
}

// New instantiates a new GCP provisioner
func New(config *configuration.Configuration) *GCP {
	b := packr.New("GCP", projectPath)
	return &GCP{
		box:    b,
		config: config,
	}
}

// SetTerraformExecutor adds the terraform executor to this provisioner
func (d *GCP) SetTerraformExecutor(tf *tfexec.Terraform) {
	d.terraform = tf
}

// TerraformExecutor returns the current terraform executor of this provisioner
func (d *GCP) TerraformExecutor() (tf *tfexec.Terraform) {
	return d.terraform
}

// Box returns the box that has the files as binary data
func (d GCP) Box() *packr.Box {
	return d.box
}

// TerraformFiles returns the list of files conforming the terraform project
func (d GCP) TerraformFiles() []string {
	// TODO understand if it is possible to deduce these values somehow
	// find . -type f -follow -print
	return []string{
		"output.tf",
		"main.tf",
		"variables.tf",
	}
}

// Plan runs a dry run execution
func (d GCP) Plan() (err error) {
	log.Info("[DRYRUN] Updating GCP Bootstrap project")
	err = d.createVarFile()
	if err != nil {
		return err
	}
	changes, err := d.terraform.Plan(context.Background(), tfexec.VarFile(fmt.Sprintf("%v/gcp.tfvars", d.terraform.WorkingDir())))
	if err != nil {
		log.Fatalf("[DRYRUN] Something went wrong while updating gcp. %v", err)
		return err
	}
	if changes {
		log.Warn("[DRYRUN] Something changed along the time. Remove dryrun option to apply the desired state")
	} else {
		log.Info("[DRYRUN] Everything is up to date")
	}

	log.Info("[DRYRUN] GCP Updated")
	return nil
}

func (d GCP) Prepare() (err error) {
	return nil
}

// Update runs terraform apply in the project
func (d GCP) Update() (string, error) {
	log.Info("Updating GCP Bootstrap project")
	err := d.createVarFile()
	if err != nil {
		return "", err
	}

	err = d.terraform.Apply(context.Background(), tfexec.VarFile(fmt.Sprintf("%v/gcp.tfvars", d.terraform.WorkingDir())))
	if err != nil {
		log.Fatalf("Something went wrong while updating gcp. %v", err)
		return "", err
	}

	log.Info("GCP Updated")
	return "", nil
}

// Destroy runs terraform destroy in the project
func (d GCP) Destroy() (err error) {
	log.Info("Destroying GCP Bootstrap project")
	err = d.createVarFile()
	if err != nil {
		return err
	}

	err = d.terraform.Destroy(context.Background(), tfexec.VarFile(fmt.Sprintf("%v/gcp.tfvars", d.terraform.WorkingDir())))
	if err != nil {
		log.Fatalf("Something went wrong while destroying GCP Bootstrap project. %v", err)
		return err
	}
	log.Info("GCP Bootstrap destroyed")
	return nil
}
