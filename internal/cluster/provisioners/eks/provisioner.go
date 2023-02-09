// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eks

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
func (e *EKS) InitMessage() string {
	return `Kubernetes Fury EKS

This provisioner creates a battle-tested Kubernetes Fury Cluster based on AWS EKS
with a private control plane and a production-grade setup.

Requires network connectivity to the target VPC (like a VPN connection) to deploy the cluster.
Use a bastion host (inside the EKS VPC) as an alternative method to deploy the cluster.

The provisioner requires the following software installed:
- /bin/sh
- wget or curl
- aws-cli
- kubectl
`
}

// UpdateMessage return a custom provisioner message the user will see once the cluster is updated
func (e *EKS) UpdateMessage() string {
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
		`Kubernetes Fury EKS

All the cluster components are up to date.

Kubernetes Fury EKS cluster is ready.

EKS Cluster Endpoint: %v
SSH Operator Name: %v

Use the ssh %v username to access the EKS instances with the configured SSH key.

Discover the instances by running

$ kubectl get nodes

Then access them by running:

$ ssh %v@<node-name-reported-by-kubectl-get-nodes>

`, clusterEndpoint, clusterOperatorName, clusterOperatorName, clusterOperatorName,
	)
}

// DestroyMessage return a custom provisioner message the user will see once the cluster is destroyed
func (e *EKS) DestroyMessage() string {
	return `Kubernetes Fury EKS

All cluster components were destroyed.

EKS control plane and workers went away.

If you faced any problems, please contact support if you have a subscription or write us to sales@sighup.io.
`
}

// Enterprise return a boolean indicating it is an enterprise provisioner
func (e *EKS) Enterprise() bool {
	return false
}

// EKS represents the EKS provisioner
type EKS struct {
	terraform *tfexec.Terraform
	box       *packr.Box
	config    *configuration.Configuration
}

const (
	projectPath = "../../../../data/provisioners/cluster/eks"
)

func (e EKS) createVarFile() (err error) {
	var buffer bytes.Buffer
	spec := e.config.Spec.(cfg.EKS)
	buffer.WriteString(fmt.Sprintf("cluster_name = \"%v\"\n", e.config.Metadata.Name))
	buffer.WriteString(fmt.Sprintf("cluster_version = \"%v\"\n", spec.Version))
	if spec.LogRetentionDays != 0 {
		buffer.WriteString(fmt.Sprintf("cluster_log_retention_days = %v\n", spec.LogRetentionDays))
	}
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

	if len(spec.Auth.AdditionalAccounts) > 0 {
		buffer.WriteString(
			fmt.Sprintf(
				"eks_map_accounts = [\"%v\"]\n",
				strings.Join(spec.Auth.AdditionalAccounts, "\",\""),
			),
		)
	}

	if len(spec.Auth.Users) > 0 {
		buffer.WriteString("eks_map_users = [\n")
		for _, account := range spec.Auth.Users {
			buffer.WriteString(
				fmt.Sprintf(
					`{
	groups = ["%v"]
	username = "%v"
	userarn = "%v"
},
`, strings.Join(account.Groups, "\",\""), account.Username, account.UserARN,
				),
			)
		}
		buffer.WriteString("]\n")

	}

	if len(spec.Auth.Roles) > 0 {
		buffer.WriteString("eks_map_roles = [\n")
		for _, account := range spec.Auth.Roles {
			buffer.WriteString(
				fmt.Sprintf(
					`{
	groups = ["%v"]
	username = "%v"
	rolearn = "%v"
},
`, strings.Join(account.Groups, "\",\""), account.Username, account.RoleARN,
				),
			)
		}
		buffer.WriteString("]\n")
	}

	if len(spec.NodePools) > 0 {
		buffer.WriteString("node_pools = [\n")
		for _, np := range spec.NodePools {
			buffer.WriteString("{\n")
			buffer.WriteString(fmt.Sprintf("name = \"%v\"\n", np.Name))
			if np.Version != "" {
				buffer.WriteString(fmt.Sprintf("version = \"%v\"\n", np.Version))
			} else {
				buffer.WriteString("version = null\n")
			}
			if np.ContainerRuntime != "" {
				buffer.WriteString(fmt.Sprintf("container_runtime = \"%v\"\n", np.ContainerRuntime))
			}
			buffer.WriteString(fmt.Sprintf("spot_instance = %v\n", np.SpotInstance))
			buffer.WriteString(fmt.Sprintf("min_size = %v\n", np.MinSize))
			buffer.WriteString(fmt.Sprintf("max_size = %v\n", np.MaxSize))
			buffer.WriteString(fmt.Sprintf("instance_type = \"%v\"\n", np.InstanceType))

			if np.OS != "" {
				buffer.WriteString(fmt.Sprintf("os = \"%v\"\n", np.OS))
			}
			if len(np.TargetGroups) > 0 {
				buffer.WriteString(fmt.Sprintf("eks_target_group_arns = [\"%v\"]\n", strings.Join(np.TargetGroups, "\",\"")))
			}

			if np.MaxPods > 0 {
				buffer.WriteString(fmt.Sprintf("max_pods = %v\n", np.MaxPods))
			}
			buffer.WriteString(fmt.Sprintf("volume_size = %v\n", np.VolumeSize))

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

			if len(np.SubNetworks) > 0 {
				buffer.WriteString(fmt.Sprintf("subnetworks = [\"%v\"]\n", strings.Join(np.SubNetworks, "\",\"")))
			} else {
				buffer.WriteString("subnetworks = null\n")
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

			buffer.WriteString("},\n")
		}
		buffer.WriteString("]\n")
	}
	// For this version we will check for this field value to be present, otherwise we could trigger an unwanted rollout of the node pools for existing clusters.
	// The switch from launch configurations to launch templates for the Node Pools requires some manual intervention.
	// We could automate this away though.
	if spec.NodePoolsLaunchKind == "" {
		log.Fatalf(".spec.nodePoolsKind is not set in the cluster configuration file. Please set it explicitly to `launch_configurations`, `launch_templates` or `both` to avoid unwanted node pools rollouts. For new clusters you can use `launch_templates`, for existing clusters please check the Fury EKS Installer docs: https://github.com/sighupio/fury-eks-installer/blob/master/docs/upgrades/v1.9-to-v1.10.0.md")
	} else {
		buffer.WriteString(fmt.Sprintf("node_pools_launch_kind = \"%v\"\n", spec.NodePoolsLaunchKind))
	}
	err = ioutil.WriteFile(fmt.Sprintf("%v/eks.tfvars", e.terraform.WorkingDir()), buffer.Bytes(), 0600)
	if err != nil {
		return err
	}
	err = e.terraform.FormatWrite(
		context.Background(),
		tfexec.Dir(fmt.Sprintf("%v/eks.tfvars", e.terraform.WorkingDir())),
	)
	if err != nil {
		return err
	}
	return nil
}

// New instantiates a new EKS provisioner
func New(config *configuration.Configuration) *EKS {
	b := packr.New("ekscluster", projectPath)
	return &EKS{
		box:    b,
		config: config,
	}
}

// SetTerraformExecutor adds the terraform executor to this provisioner
func (e *EKS) SetTerraformExecutor(tf *tfexec.Terraform) {
	e.terraform = tf
}

// TerraformExecutor returns the current terraform executor of this provisioner
func (e *EKS) TerraformExecutor() (tf *tfexec.Terraform) {
	return e.terraform
}

// Box returns the box that has the files as binary data
func (e EKS) Box() *packr.Box {
	return e.box
}

// TerraformFiles returns the list of files conforming the terraform project
func (e EKS) TerraformFiles() []string {
	// TODO understand if it is possible to deduce these values somehow
	// find . -type f -follow -print
	return []string{
		"output.tf",
		"main.tf",
		"variables.tf",
	}
}

func (e EKS) Prepare() (err error) {
	return nil
}

// Plan runs a dry run execution
func (e EKS) Plan() (err error) {
	log.Info("[DRYRUN] Updating EKS Cluster project")
	err = e.createVarFile()
	if err != nil {
		return err
	}
	var changes bool
	changes, err = e.terraform.Plan(
		context.Background(),
		tfexec.VarFile(fmt.Sprintf("%v/eks.tfvars", e.terraform.WorkingDir())),
	)
	if err != nil {
		log.Fatalf("[DRYRUN] Got error while updating EKS: %v", err)
		return err
	}
	if changes {
		log.Warn("[DRYRUN] Something has changed in the mean time. Remove --dry-run flag to apply the desired state")
	} else {
		log.Info("[DRYRUN] Everything is up to date")
	}

	log.Info("[DRYRUN] EKS Updated")
	return nil
}

// Update runs terraform apply in the project
func (e EKS) Update() (string, error) {
	log.Info("Updating EKS project")
	err := e.createVarFile()
	if err != nil {
		return "", err
	}
	err = e.terraform.Apply(
		context.Background(),
		tfexec.VarFile(fmt.Sprintf("%v/eks.tfvars", e.terraform.WorkingDir())),
	)
	if err != nil {
		log.Fatalf("Got error while updating EKS: %v", err)
		return "", err
	}

	log.Info("EKS Updated")
	return e.kubeconfig()
}

// Destroy runs terraform destroy in the project
func (e EKS) Destroy() (err error) {
	log.Info("Destroying EKS project")
	err = e.createVarFile()
	if err != nil {
		return err
	}
	err = e.terraform.Destroy(
		context.Background(),
		tfexec.VarFile(fmt.Sprintf("%v/eks.tfvars", e.terraform.WorkingDir())),
	)
	if err != nil {
		log.Fatalf("Got error while destroying EKS cluster project: %v", err)
		return err
	}
	log.Info("EKS destroyed")
	return nil
}

func (e EKS) kubeconfig() (string, error) {
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
	return creds, err
}
