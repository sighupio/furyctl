// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/sighupio/fury-distribution/pkg/config"
	"github.com/sighupio/fury-distribution/pkg/schema"
	"github.com/sighupio/furyctl/configs"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/template"
	"github.com/sighupio/furyctl/internal/tool/terraform"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
)

type Kubernetes struct {
	*cluster.CreationPhase
	furyctlConf      schema.EksclusterKfdV1Alpha2
	kfdManifest      config.KFD
	infraOutputsPath string
	tfRunner         *terraform.Runner
}

func NewKubernetes(
	furyctlConf schema.EksclusterKfdV1Alpha2,
	kfdManifest config.KFD,
	infraOutputsPath string,
) (*Kubernetes, error) {
	phase, err := cluster.NewCreationPhase(".kubernetes")
	if err != nil {
		return nil, err
	}

	return &Kubernetes{
		CreationPhase:    phase,
		furyctlConf:      furyctlConf,
		kfdManifest:      kfdManifest,
		infraOutputsPath: infraOutputsPath,
		tfRunner: terraform.NewRunner(
			execx.NewStdExecutor(),
			terraform.Paths{
				Logs:      phase.LogsPath,
				Outputs:   phase.OutputsPath,
				WorkDir:   path.Join(phase.Path, "terraform"),
				Plan:      phase.PlanPath,
				Terraform: phase.TerraformPath,
			},
		),
	}, nil
}

func (k *Kubernetes) Exec(dryRun bool) error {
	timestamp := time.Now().Unix()

	if err := k.CreateFolder(); err != nil {
		return err
	}

	if err := k.copyFromTemplate(); err != nil {
		return err
	}

	if err := k.CreateFolderStructure(); err != nil {
		return err
	}

	if err := k.createTfVars(); err != nil {
		return err
	}

	if err := k.tfRunner.Init(); err != nil {
		return err
	}

	if err := k.tfRunner.Plan(timestamp); err != nil {
		return err
	}

	if dryRun {
		return nil
	}

	out, err := k.tfRunner.Apply(timestamp)
	if err != nil {
		return err
	}

	if err := k.createKubeconfig(out); err != nil {
		return err
	}

	return k.setKubeconfigEnv()
}

func (k *Kubernetes) copyFromTemplate() error {
	var cfg template.Config

	tmpFolder, err := os.MkdirTemp("", "furyctl-kube-configs-")
	if err != nil {
		return err
	}

	defer os.RemoveAll(tmpFolder)

	subFS, err := fs.Sub(configs.Tpl, path.Join("provisioners", "cluster", "eks"))
	if err != nil {
		return err
	}

	err = iox.CopyRecursive(subFS, tmpFolder)
	if err != nil {
		return err
	}

	targetTfDir := path.Join(k.Path, "terraform")
	prefix := "kube"
	tfConfVars := map[string]map[any]any{
		"kubernetes": {
			"eks": k.kfdManifest.Kubernetes.Eks,
		},
	}

	cfg.Data = tfConfVars

	return k.CreationPhase.CopyFromTemplate(
		cfg,
		prefix,
		tmpFolder,
		targetTfDir,
	)
}

func (k *Kubernetes) createKubeconfig(o terraform.OutputJson) error {
	if o.Outputs["kubeconfig"] == nil {
		return fmt.Errorf("can't get kubeconfig from terraform apply logs")
	}

	kubeString, ok := o.Outputs["kubeconfig"].Value.(string)
	if !ok {
		return fmt.Errorf("can't get kubeconfig from terraform apply logs")
	}

	return os.WriteFile(path.Join(k.SecretsPath, "kubeconfig"), []byte(kubeString), 0o600)
}

func (k *Kubernetes) setKubeconfigEnv() error {
	kubePath, err := filepath.Abs(path.Join(k.SecretsPath, "kubeconfig"))
	if err != nil {
		return err
	}

	return os.Setenv("KUBECONFIG", kubePath)
}

//nolint:gocyclo,maintidx // it will be refactored
func (k *Kubernetes) createTfVars() error {
	var buffer bytes.Buffer

	subnetIdsSource := k.furyctlConf.Spec.Kubernetes.SubnetIds
	vpcIdSource := k.furyctlConf.Spec.Kubernetes.VpcId
	allowedCidrsSource := k.furyctlConf.Spec.Kubernetes.ApiServerEndpointAccess.AllowedCidrs

	if infraOutJson, err := os.ReadFile(path.Join(k.infraOutputsPath, "output.json")); err == nil {
		var infraOut terraform.OutputJson

		if err := json.Unmarshal(infraOutJson, &infraOut); err == nil {
			if infraOut.Outputs["private_subnets"] == nil {
				return fmt.Errorf("private_subnets not found in infra output")
			}

			s, ok := infraOut.Outputs["private_subnets"].Value.([]interface{})
			if !ok {
				return fmt.Errorf("cannot read private_subnets from infrastructure's output.json")
			}

			if infraOut.Outputs["vpc_id"] == nil {
				return fmt.Errorf("vpc_id not found in infra output")
			}

			v, ok := infraOut.Outputs["vpc_id"].Value.(string)
			if !ok {
				return fmt.Errorf("cannot read vpc_id from infrastructure's output.json")
			}

			if infraOut.Outputs["vpc_cidr_block"] == nil {
				return fmt.Errorf("vpc_cidr_block not found in infra output")
			}

			c, ok := infraOut.Outputs["vpc_cidr_block"].Value.(string)
			if !ok {
				return fmt.Errorf("cannot read vpc_cidr_block from infrastructure's output.json")
			}

			subs := make([]schema.TypesAwsSubnetId, len(s))
			for i, sub := range s {
				ss, ok := sub.(string)
				if !ok {
					return fmt.Errorf("cannot read private_subnets from infrastructure's output.json")
				}

				subs[i] = schema.TypesAwsSubnetId(ss)
			}

			subnetIdsSource = subs
			vpcIdSource = schema.TypesAwsVpcId(v)
			allowedCidrsSource = []schema.TypesCidr{schema.TypesCidr(c)}
		}
	}

	buffer.WriteString(fmt.Sprintf("cluster_name = \"%v\"\n", k.furyctlConf.Metadata.Name))
	buffer.WriteString(fmt.Sprintf("cluster_version = \"%v\"\n", k.kfdManifest.Kubernetes.Eks.Version))
	buffer.WriteString(fmt.Sprintf("network = \"%v\"\n", vpcIdSource))

	subnetIds := make([]string, len(subnetIdsSource))

	for i, subnetId := range subnetIdsSource {
		subnetIds[i] = fmt.Sprintf("\"%v\"", subnetId)
	}

	buffer.WriteString(fmt.Sprintf("subnetworks = [%v]\n", strings.Join(subnetIds, ",")))

	dmzCidrRange := make([]string, len(allowedCidrsSource))

	for i, cidr := range allowedCidrsSource {
		dmzCidrRange[i] = fmt.Sprintf("\"%v\"", cidr)
	}

	buffer.WriteString(fmt.Sprintf("dmz_cidr_range = [%v]\n", strings.Join(dmzCidrRange, ",")))
	buffer.WriteString(fmt.Sprintf("ssh_public_key = \"%v\"\n", k.furyctlConf.Spec.Kubernetes.NodeAllowedSshPublicKey))
	if k.furyctlConf.Spec.Tags != nil && len(k.furyctlConf.Spec.Tags) > 0 {
		var tags []byte
		tags, err := json.Marshal(k.furyctlConf.Spec.Tags)
		if err != nil {
			return err
		}
		buffer.WriteString(fmt.Sprintf("tags = %v\n", string(tags)))
	}

	if len(k.furyctlConf.Spec.Kubernetes.AwsAuth.AdditionalAccounts) > 0 {
		buffer.WriteString(
			fmt.Sprintf(
				"eks_map_accounts = [\"%v\"]\n",
				strings.Join(k.furyctlConf.Spec.Kubernetes.AwsAuth.AdditionalAccounts, "\",\""),
			),
		)
	}

	if len(k.furyctlConf.Spec.Kubernetes.AwsAuth.Users) > 0 {
		buffer.WriteString("eks_map_users = [\n")
		for _, account := range k.furyctlConf.Spec.Kubernetes.AwsAuth.Users {
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

	if len(k.furyctlConf.Spec.Kubernetes.AwsAuth.Roles) > 0 {
		buffer.WriteString("eks_map_roles = [\n")
		for _, account := range k.furyctlConf.Spec.Kubernetes.AwsAuth.Roles {
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

	if len(k.furyctlConf.Spec.Kubernetes.NodePools) > 0 {
		buffer.WriteString("node_pools = [\n")
		for _, np := range k.furyctlConf.Spec.Kubernetes.NodePools {
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

	targetTfVars := path.Join(k.Path, "terraform", "main.auto.tfvars")

	return os.WriteFile(targetTfVars, buffer.Bytes(), 0o600)
}
