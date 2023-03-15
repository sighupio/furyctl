// Copyright (c) 2022 SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gke

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/gobuffalo/packr/v2"
	"github.com/hashicorp/terraform-exec/tfexec"
	log "github.com/sirupsen/logrus"

	cfg "github.com/sighupio/furyctl/internal/cluster/configuration"
	"github.com/sighupio/furyctl/internal/configuration"
)

// InitMessage return a custom provisioner message the user will see once the cluster is ready to be updated
func (e *GKE) InitMessage() string {
	return `[GKE] Fury

This provisioner creates a battle-tested Google Cloud GKE Kubernetes Cluster
with a private and production-grade setup.

Requires to connect to a VPN server to deploy the cluster from this computer.
Use a bastion host (inside the GKE VPC) as an alternative method to deploy the cluster.

The provisioner requires the following software installed:
- /bin/sh
- wget or curl
- gcloud
- kubectl
`
}

// UpdateMessage return a custom provisioner message the user will see once the cluster is updated
func (e *GKE) UpdateMessage() string {
	var output map[string]tfexec.OutputMeta
	output, err := e.terraform.Output(context.Background())
	if err != nil {
		log.Error("Can not get output values")
	}
	var clusterEndpoint, clusterOperatorName string
	err = json.Unmarshal(output["cluster_endpoint"].Value, &clusterEndpoint)
	if err != nil {
		log.Error("Can not get `cluster_endpoint` value")
	}
	err = json.Unmarshal(output["operator_ssh_user"].Value, &clusterOperatorName)
	if err != nil {
		log.Error("Can not get `operator_ssh_user` value")
	}
	return fmt.Sprintf(
		`[GKE] Fury

All the cluster components are up to date.
GKE Kubernetes cluster ready.

GKE Cluster Endpoint: %v
SSH Operator Name: %v

Use the ssh %v username to access the GKE instances with the configured SSH key.
Discover the instances by running

$ kubectl get nodes

Then access by running:

$ ssh %v@node-name-reported-by-kubectl-get-nodes

`, clusterEndpoint, clusterOperatorName, clusterOperatorName, clusterOperatorName,
	)
}

// DestroyMessage return a custom provisioner message the user will see once the cluster is destroyed
func (e *GKE) DestroyMessage() string {
	return `[GKE] Fury
All cluster components were destroyed.
GKE control plane and workers went away.

Had problems, contact us at sales@sighup.io.
`
}

// Enterprise return a boolean indicating it is an enterprise provisioner
func (e *GKE) Enterprise() bool {
	return false
}

// GKE represents the GKE provisioner
type GKE struct {
	terraform *tfexec.Terraform
	box       *packr.Box
	config    *configuration.Configuration
}

const (
	projectPath = "../../../../data/provisioners/cluster/gke"
)

func (e GKE) createVarFile() (err error) {
	var buffer bytes.Buffer
	spec := e.config.Spec.(cfg.GKE)

	buffer.WriteString(fmt.Sprintf("provider_region = \"%v\"\n", spec.Region))
	buffer.WriteString(fmt.Sprintf("provider_project = \"%v\"\n", spec.Project))

	buffer.WriteString(fmt.Sprintf("cluster_name = \"%v\"\n", e.config.Metadata.Name))
	buffer.WriteString(fmt.Sprintf("cluster_version = \"%v\"\n", spec.Version))
	buffer.WriteString(fmt.Sprintf("network = \"%v\"\n", spec.Network))
	buffer.WriteString(fmt.Sprintf("subnetworks = [\"%v\"]\n", strings.Join(spec.SubNetworks, "\",\"")))
	buffer.WriteString(fmt.Sprintf("dmz_cidr_range = [\"%v\"]\n", strings.Join(spec.DMZCIDRRange.Values, "\",\"")))
	buffer.WriteString(fmt.Sprintf("ssh_public_key = \"%v\"\n", spec.SSHPublicKey))
	if len(spec.Tags) > 0 {
		var tags []byte
		tags, err = json.Marshal(spec.Tags)
		if err != nil {
			return err
		}
		buffer.WriteString(fmt.Sprintf("tags = %v\n", string(tags)))
	}

	if len(spec.NodePools) > 0 {
		buffer.WriteString("node_pools = [\n")
		for _, np := range spec.NodePools {
			buffer.WriteString("{\n")
			buffer.WriteString(fmt.Sprintf("name = \"%v\"\n", np.Name))
			buffer.WriteString(fmt.Sprintf("version = \"%v\"\n", np.Version))
			buffer.WriteString(fmt.Sprintf("min_size = %v\n", np.MinSize))
			buffer.WriteString(fmt.Sprintf("max_size = %v\n", np.MaxSize))
			buffer.WriteString(fmt.Sprintf("instance_type = \"%v\"\n", np.InstanceType))
			if np.OS != "" {
				buffer.WriteString(fmt.Sprintf("os = \"%v\"\n", np.OS))
			}
			if np.MaxPods > 0 {
				buffer.WriteString(fmt.Sprintf("max_pods = %v\n", np.MaxPods))
			}
			buffer.WriteString(fmt.Sprintf("volume_size = %v\n", np.VolumeSize))
			buffer.WriteString(fmt.Sprintf("spot_instance = %v\n", np.SpotInstance))

			if len(np.SubNetworks) > 0 {
				buffer.WriteString(fmt.Sprintf("subnetworks = [\"%v\"]\n", strings.Join(np.SubNetworks, "\",\"")))
			} else {
				buffer.WriteString("subnetworks = []\n")
			}
			if len(np.Labels) > 0 {
				var labels []byte
				labels, err = json.Marshal(np.Labels)
				if err != nil {
					return err
				}
				buffer.WriteString(fmt.Sprintf("labels = %v\n", string(labels)))
			} else {
				buffer.WriteString("labels = {}\n")
			}

			if len(np.Taints) > 0 {
				buffer.WriteString(fmt.Sprintf("taints = [\"%v\"]\n", strings.Join(np.Taints, "\",\"")))
			} else {
				buffer.WriteString("taints = []\n")
			}

			if len(np.Tags) > 0 {
				var tags []byte
				tags, err = json.Marshal(np.Tags)
				if err != nil {
					return err
				}
				buffer.WriteString(fmt.Sprintf("tags = %v\n", string(tags)))
			} else {
				buffer.WriteString("tags = {}\n")
			}

			if len(np.AdditionalFirewallRules) > 0 {

				buffer.WriteString("additional_firewall_rules = [\n")
				for _, fwRule := range np.AdditionalFirewallRules {

					fwRuleTags := "{}"
					if len(fwRule.Tags) > 0 {
						var tags []byte
						tags, err = json.Marshal(fwRule.Tags)
						if err != nil {
							return err
						}
						fwRuleTags = string(tags)
					}

					buffer.WriteString(
						fmt.Sprintf(
							`{
			name = "%v"
			direction = "%v"
			cidr_block = "%v"
			protocol = "%v"
			ports = "%v"
			tags = %v
		},
		`, fwRule.Name, fwRule.Direction, fwRule.CIDRBlock, fwRule.Protocol, fwRule.Ports, fwRuleTags,
						),
					)
				}
				buffer.WriteString("]\n")
			} else {
				buffer.WriteString("additional_firewall_rules = []\n")
			}

			buffer.WriteString("},\n")
		}
		buffer.WriteString("]\n")
	}

	buffer.WriteString(fmt.Sprintf("gke_network_project_id = \"%v\"\n", spec.NetworkProjectID))
	buffer.WriteString(fmt.Sprintf("gke_master_ipv4_cidr_block = \"%v\"\n", spec.ControlPlaneCIDR))
	buffer.WriteString(fmt.Sprintf("gke_add_additional_firewall_rules = %v\n", spec.AdditionalFirewallRules))
	buffer.WriteString(fmt.Sprintf("gke_add_cluster_firewall_rules = %v\n", spec.AdditionalClusterFirewallRules))
	buffer.WriteString(fmt.Sprintf("gke_disable_default_snat = %v\n", spec.DisableDefaultSNAT))

	err = ioutil.WriteFile(fmt.Sprintf("%v/gke.tfvars", e.terraform.WorkingDir()), buffer.Bytes(), 0600)
	if err != nil {
		return err
	}
	err = e.terraform.FormatWrite(
		context.Background(),
		tfexec.Dir(fmt.Sprintf("%v/gke.tfvars", e.terraform.WorkingDir())),
	)
	if err != nil {
		return err
	}
	return nil
}

// New instantiates a new GKE provisioner
func New(config *configuration.Configuration) *GKE {
	b := packr.New("gkecluster", projectPath)
	return &GKE{
		box:    b,
		config: config,
	}
}

// SetTerraformExecutor adds the terraform executor to this provisioner
func (e *GKE) SetTerraformExecutor(tf *tfexec.Terraform) {
	e.terraform = tf
}

// TerraformExecutor returns the current terraform executor of this provisioner
func (e *GKE) TerraformExecutor() (tf *tfexec.Terraform) {
	return e.terraform
}

// Box returns the box that has the files as binary data
func (e GKE) Box() *packr.Box {
	return e.box
}

// TerraformFiles returns the list of files conforming the terraform project
func (e GKE) TerraformFiles() []string {
	// TODO understand if it is possible to deduce these values somehow
	// find . -type f -follow -print
	return []string{
		"output.tf",
		"main.tf",
		"variables.tf",
	}
}

// Plan runs a dry run execution
func (e GKE) Plan() (err error) {
	log.Info("[DRYRUN] Updating GKE Cluster project")
	err = e.createVarFile()
	if err != nil {
		return err
	}
	var changes bool
	changes, err = e.terraform.Plan(
		context.Background(),
		tfexec.VarFile(fmt.Sprintf("%v/gke.tfvars", e.terraform.WorkingDir())),
	)
	if err != nil {
		log.Fatalf("[DRYRUN] Something went wrong while updating gke. %v", err)
		return err
	}
	if changes {
		log.Warn("[DRYRUN] Something changed along the time. Remove dryrun option to apply the desired state")
	} else {
		log.Info("[DRYRUN] Everything is up to date")
	}

	log.Info("[DRYRUN] GKE Updated")
	return nil
}

func (e GKE) Prepare() (err error) {
	return nil
}

// Update runs terraform apply in the project
func (e GKE) Update() (string, error) {
	log.Info("Updating GKE project")
	err := e.createVarFile()
	if err != nil {
		return "", err
	}
	err = e.terraform.Apply(
		context.Background(),
		tfexec.VarFile(fmt.Sprintf("%v/gke.tfvars", e.terraform.WorkingDir())),
	)
	if err != nil {
		log.Fatalf("Something went wrong while updating gke. %v", err)
		return "", err
	}

	log.Info("GKE Updated")
	return e.kubeconfig()
}

// Destroy runs terraform destroy in the project
func (e GKE) Destroy() (err error) {
	log.Info("Destroying GKE project")
	err = e.createVarFile()
	if err != nil {
		return err
	}
	err = e.terraform.Destroy(
		context.Background(),
		tfexec.VarFile(fmt.Sprintf("%v/gke.tfvars", e.terraform.WorkingDir())),
	)
	if err != nil {
		log.Fatalf("Something went wrong while destroying GKE cluster project. %v", err)
		return err
	}
	log.Info("GKE destroyed")
	return nil
}

func (e GKE) kubeconfig() (string, error) {
	log.Info("Gathering output file as json")
	var output map[string]tfexec.OutputMeta
	output, err := e.terraform.Output(context.Background())
	if err != nil {
		log.Fatalf("Error while getting project output: %v", err)
		return "", err
	}
	var creds string
	err = json.Unmarshal(output["kubeconfig"].Value, &creds)
	if err != nil {
		log.Fatalf("Error while tranforming the kubeconfig value into string: %v", err)
		return "", err
	}
	return creds, nil
}
