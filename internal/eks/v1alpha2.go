// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/config"
	"github.com/sighupio/fury-distribution/pkg/schema"
)

var ErrUnsupportedPhase = fmt.Errorf("unsupported phase")

type V1alpha2 struct {
	Phase          string
	KfdManifest    config.KFD
	FuryFile       schema.EksclusterKfdV1Alpha2
	ConfigPath     string
	VpnAutoConnect bool
}

func (v *V1alpha2) Create(dryRun bool) error {
	logrus.Infof("Running phase: %s", v.Phase)

	switch v.Phase {
	case "infrastructure":
		return v.Infrastructure(dryRun)
	case "kubernetes":
		return v.Kubernetes(dryRun)
	case "distribution":
		return v.Distribution(dryRun)
	case "":
		if v.FuryFile.Spec.Distribution != nil {
			err := v.Infrastructure(dryRun)
			if err != nil {
				return err
			}
		}

		err := v.Kubernetes(dryRun)
		if err != nil {
			return err
		}

		return v.Distribution(dryRun)
	default:
		return ErrUnsupportedPhase
	}
}

func (v *V1alpha2) Infrastructure(dryRun bool) error {
	timestamp := time.Now().Unix()

	infra, err := NewInfrastructure()
	if err != nil {
		return err
	}

	err = infra.CreateFolder()
	if err != nil {
		return err
	}

	err = infra.CopyFromTemplate(v.KfdManifest)
	if err != nil {
		return err
	}

	err = infra.CreateFolderStructure()
	if err != nil {
		return err
	}

	err = v.CreateInfraTfVars(infra.Path)
	if err != nil {
		return err
	}

	err = infra.TerraformInit()
	if err != nil {
		return err
	}

	err = infra.TerraformPlan(timestamp)
	if err != nil {
		return err
	}

	if !dryRun {
		_, err = infra.TerraformApply(timestamp)
		if err != nil {
			return err
		}

		if v.FuryFile.Spec.Infrastructure.Vpc.Vpn != nil && v.FuryFile.Spec.Infrastructure.Vpc.Vpn.Instances > 0 {
			clientName := v.FuryFile.Metadata.Name

			whoamiResp, err := exec.Command("whoami").Output()
			if err != nil {
				return err
			}

			whoami := strings.TrimSpace(string(whoamiResp))
			clientName = fmt.Sprintf("%s-%s", clientName, whoami)

			err = infra.CreateOvpnFile(clientName)
			if err != nil {
				return err
			}

			if v.VpnAutoConnect {
				err = infra.CreateOvpnConnection(clientName)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (v *V1alpha2) Kubernetes(dryRun bool) error {
	timestamp := time.Now().Unix()

	kube, err := NewKubernetes()
	if err != nil {
		return err
	}

	err = kube.CreateFolder()
	if err != nil {
		return err
	}

	err = kube.CopyFromTemplate(v.KfdManifest)
	if err != nil {
		return err
	}

	err = kube.CreateFolderStructure()
	if err != nil {
		return err
	}

	err = v.CreateKubernetesTfVars(kube.Path)
	if err != nil {
		return err
	}

	err = kube.TerraformInit()
	if err != nil {
		return err
	}

	err = kube.TerraformPlan(timestamp)
	if err != nil {
		return err
	}

	if !dryRun {
		out, err := kube.TerraformApply(timestamp)
		if err != nil {
			return err
		}

		err = kube.CreateKubeconfig(out)
		if err != nil {
			return err
		}

		err = kube.SetKubeconfigEnv()
		if err != nil {
			return err
		}
	}

	return nil
}

func (v *V1alpha2) Distribution(dryRun bool) error {
	return nil
}

func (v *V1alpha2) CreateInfraTfVars(infraPath string) error {
	var buffer bytes.Buffer

	buffer.WriteString(fmt.Sprintf("name = \"%v\"\n", v.FuryFile.Metadata.Name))
	buffer.WriteString(fmt.Sprintf(
		"network_cidr = \"%v\"\n",
		v.FuryFile.Spec.Infrastructure.Vpc.Network.Cidr,
	))

	publicSubnetworkCidrs := make([]string, len(v.FuryFile.Spec.Infrastructure.Vpc.Network.SubnetsCidrs.Public))

	for i, cidr := range v.FuryFile.Spec.Infrastructure.Vpc.Network.SubnetsCidrs.Public {
		publicSubnetworkCidrs[i] = fmt.Sprintf("\"%v\"", cidr)
	}

	privateSubnetworkCidrs := make([]string, len(v.FuryFile.Spec.Infrastructure.Vpc.Network.SubnetsCidrs.Private))

	for i, cidr := range v.FuryFile.Spec.Infrastructure.Vpc.Network.SubnetsCidrs.Private {
		privateSubnetworkCidrs[i] = fmt.Sprintf("\"%v\"", cidr)
	}

	buffer.WriteString(fmt.Sprintf(
		"public_subnetwork_cidrs = [%v]\n",
		strings.Join(publicSubnetworkCidrs, ",")))

	buffer.WriteString(fmt.Sprintf(
		"private_subnetwork_cidrs = [%v]\n",
		strings.Join(privateSubnetworkCidrs, ",")))

	if v.FuryFile.Spec.Infrastructure.Vpc.Vpn != nil {
		buffer.WriteString(
			fmt.Sprintf(
				"vpn_subnetwork_cidr = \"%v\"\n",
				v.FuryFile.Spec.Infrastructure.Vpc.Vpn.VpnClientsSubnetCidr,
			),
		)
		buffer.WriteString(
			fmt.Sprintf(
				"vpn_instances = %v\n",
				v.FuryFile.Spec.Infrastructure.Vpc.Vpn.Instances,
			),
		)

		if v.FuryFile.Spec.Infrastructure.Vpc.Vpn.Port != 0 {
			buffer.WriteString(
				fmt.Sprintf(
					"vpn_port = %v\n",
					v.FuryFile.Spec.Infrastructure.Vpc.Vpn.Port,
				),
			)
		}

		if v.FuryFile.Spec.Infrastructure.Vpc.Vpn.InstanceType != "" {
			buffer.WriteString(
				fmt.Sprintf(
					"vpn_instance_type = \"%v\"\n",
					v.FuryFile.Spec.Infrastructure.Vpc.Vpn.InstanceType,
				),
			)
		}

		if v.FuryFile.Spec.Infrastructure.Vpc.Vpn.DiskSize != 0 {
			buffer.WriteString(
				fmt.Sprintf(
					"vpn_instance_disk_size = %v\n",
					v.FuryFile.Spec.Infrastructure.Vpc.Vpn.DiskSize,
				),
			)
		}

		if v.FuryFile.Spec.Infrastructure.Vpc.Vpn.OperatorName != "" {
			buffer.WriteString(
				fmt.Sprintf(
					"vpn_operator_name = \"%v\"\n",
					v.FuryFile.Spec.Infrastructure.Vpc.Vpn.OperatorName,
				),
			)
		}

		if v.FuryFile.Spec.Infrastructure.Vpc.Vpn.DhParamsBits != 0 {
			buffer.WriteString(
				fmt.Sprintf(
					"vpn_dhparams_bits = %v\n",
					v.FuryFile.Spec.Infrastructure.Vpc.Vpn.DhParamsBits,
				),
			)
		}

		if len(v.FuryFile.Spec.Infrastructure.Vpc.Vpn.Ssh.AllowedFromCidrs) != 0 {
			allowedCidrs := make([]string, len(v.FuryFile.Spec.Infrastructure.Vpc.Vpn.Ssh.AllowedFromCidrs))

			for i, cidr := range v.FuryFile.Spec.Infrastructure.Vpc.Vpn.Ssh.AllowedFromCidrs {
				allowedCidrs[i] = fmt.Sprintf("\"%v\"", cidr)
			}

			buffer.WriteString(
				fmt.Sprintf(
					"vpn_operator_cidrs = [%v]\n",
					strings.Join(allowedCidrs, ","),
				),
			)
		}

		if len(v.FuryFile.Spec.Infrastructure.Vpc.Vpn.Ssh.GithubUsersName) != 0 {
			githubUsers := make([]string, len(v.FuryFile.Spec.Infrastructure.Vpc.Vpn.Ssh.GithubUsersName))

			for i, gu := range v.FuryFile.Spec.Infrastructure.Vpc.Vpn.Ssh.GithubUsersName {
				githubUsers[i] = fmt.Sprintf("\"%v\"", gu)
			}

			buffer.WriteString(
				fmt.Sprintf(
					"vpn_ssh_users = [%v]\n",
					strings.Join(githubUsers, ","),
				),
			)
		}
	}

	targetTfVars := path.Join(infraPath, "terraform", "main.auto.tfvars")

	return os.WriteFile(targetTfVars, buffer.Bytes(), 0o600)
}

func (v *V1alpha2) CreateKubernetesTfVars(kubePath string) error {
	var buffer bytes.Buffer

	buffer.WriteString(fmt.Sprintf("cluster_name = \"%v\"\n", v.FuryFile.Metadata.Name))
	buffer.WriteString(fmt.Sprintf("cluster_version = \"%v\"\n", v.KfdManifest.Kubernetes.Eks.Version))
	buffer.WriteString(fmt.Sprintf("network = \"%v\"\n", v.FuryFile.Spec.Kubernetes.VpcId))

	subnetIds := make([]string, len(v.FuryFile.Spec.Kubernetes.SubnetIds))

	for i, subnetId := range v.FuryFile.Spec.Kubernetes.SubnetIds {
		subnetIds[i] = fmt.Sprintf("\"%v\"", subnetId)
	}

	buffer.WriteString(fmt.Sprintf("subnetworks = [%v]\n", strings.Join(subnetIds, ",")))

	dmzCidrRange := make([]string, len(v.FuryFile.Spec.Kubernetes.ApiServerEndpointAccess.AllowedCidrs))

	for i, cidr := range v.FuryFile.Spec.Kubernetes.ApiServerEndpointAccess.AllowedCidrs {
		dmzCidrRange[i] = fmt.Sprintf("\"%v\"", cidr)
	}

	buffer.WriteString(fmt.Sprintf("dmz_cidr_range = [%v]\n", strings.Join(dmzCidrRange, ",")))
	buffer.WriteString(fmt.Sprintf("ssh_public_key = \"%v\"\n", v.FuryFile.Spec.Kubernetes.NodeAllowedSshPublicKey))
	if v.FuryFile.Spec.Tags != nil && len(v.FuryFile.Spec.Tags) > 0 {
		var tags []byte
		tags, err := json.Marshal(v.FuryFile.Spec.Tags)
		if err != nil {
			return err
		}
		buffer.WriteString(fmt.Sprintf("tags = %v\n", string(tags)))
	}

	if len(v.FuryFile.Spec.Kubernetes.AwsAuth.AdditionalAccounts) > 0 {
		buffer.WriteString(
			fmt.Sprintf(
				"eks_map_accounts = [\"%v\"]\n",
				strings.Join(v.FuryFile.Spec.Kubernetes.AwsAuth.AdditionalAccounts, "\",\""),
			),
		)
	}

	if len(v.FuryFile.Spec.Kubernetes.AwsAuth.Users) > 0 {
		buffer.WriteString("eks_map_users = [\n")
		for _, account := range v.FuryFile.Spec.Kubernetes.AwsAuth.Users {
			buffer.WriteString(
				fmt.Sprintf(
					`{
						groups = ["%v"]
						username = "%v"
						userarn = "%v"
					},`,
					strings.Join(account.Groups, "\",\""), account.Username, account.Userarn,
				),
			)
		}
		buffer.WriteString("]\n")

	}

	if len(v.FuryFile.Spec.Kubernetes.AwsAuth.Roles) > 0 {
		buffer.WriteString("eks_map_roles = [\n")
		for _, account := range v.FuryFile.Spec.Kubernetes.AwsAuth.Roles {
			buffer.WriteString(
				fmt.Sprintf(
					`{
						groups = ["%v"]
						username = "%v"
						rolearn = "%v"
					},`,
					strings.Join(account.Groups, "\",\""), account.Username, account.Rolearn,
				),
			)
		}
		buffer.WriteString("]\n")
	}

	if len(v.FuryFile.Spec.Kubernetes.NodePools) > 0 {
		buffer.WriteString("node_pools = [\n")
		for _, np := range v.FuryFile.Spec.Kubernetes.NodePools {
			buffer.WriteString("{\n")
			buffer.WriteString(fmt.Sprintf("name = \"%v\"\n", np.Name))
			buffer.WriteString("version = null\n")
			buffer.WriteString(fmt.Sprintf("spot_instance = %v\n", np.Instance.Spot))
			buffer.WriteString(fmt.Sprintf("min_size = %v\n", np.Size.Min))
			buffer.WriteString(fmt.Sprintf("max_size = %v\n", np.Size.Max))
			buffer.WriteString(fmt.Sprintf("instance_type = \"%v\"\n", np.Instance.Type))

			if len(np.AttachedTargetGroups) > 0 {
				attachedTargetGroups := make([]string, len(np.AttachedTargetGroups))

				for i, tg := range np.AttachedTargetGroups {
					attachedTargetGroups[i] = fmt.Sprintf("\"%v\"", tg)
				}

				buffer.WriteString(
					fmt.Sprintf(
						"eks_target_group_arns = [%v]\n",
						strings.Join(attachedTargetGroups, ","),
					))
			}

			buffer.WriteString(fmt.Sprintf("volume_size = %v\n", np.Instance.VolumeSize))

			if len(np.AdditionalFirewallRules) > 0 {
				buffer.WriteString("additional_firewall_rules = [\n")
				for _, fwRule := range np.AdditionalFirewallRules {
					fwRuleTags := "{}"
					if len(fwRule.Tags) > 0 {
						var tags []byte
						tags, err := json.Marshal(fwRule.Tags)
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
							},`,
							fwRule.Name,
							fwRule.Type,
							fwRule.CidrBlocks,
							fwRule.Protocol,
							fwRule.Ports,
							fwRuleTags,
						),
					)
				}
				buffer.WriteString("]\n")
			} else {
				buffer.WriteString("additional_firewall_rules = []\n")
			}

			if len(np.SubnetIds) > 0 {
				npSubNetIds := make([]string, len(np.SubnetIds))

				for i, subnetId := range np.SubnetIds {
					npSubNetIds[i] = fmt.Sprintf("\"%v\"", subnetId)
				}

				buffer.WriteString(fmt.Sprintf("subnetworks = [%v]\n", strings.Join(npSubNetIds, ",")))
			} else {
				buffer.WriteString("subnetworks = null\n")
			}
			if len(np.Labels) > 0 {
				var labels []byte
				labels, err := json.Marshal(np.Labels)
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
				tags, err := json.Marshal(np.Tags)
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

	targetTfVars := path.Join(kubePath, "terraform", "main.auto.tfvars")

	return os.WriteFile(targetTfVars, buffer.Bytes(), 0o600)
}
