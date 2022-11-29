// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package create

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/config"
	"github.com/sighupio/fury-distribution/pkg/schema"
	"github.com/sighupio/furyctl/configs"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/template"
	"github.com/sighupio/furyctl/internal/tool/terraform"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
)

var (
	errKubeconfigFromLogs = errors.New("can't get kubeconfig from terraform apply logs")
	errPvtSubnetNotFound  = errors.New("private_subnets not found in infra output")
	errPvtSubnetFromOut   = errors.New("cannot read private_subnets from infrastructure's output.json")
	errVpcCIDRFromOut     = errors.New("cannot read vpc_cidr_block from infrastructure's output.json")
	errVpcCIDRNotFound    = errors.New("vpc_cidr_block not found in infra output")
)

type Kubernetes struct {
	*cluster.OperationPhase
	furyctlConf      schema.EksclusterKfdV1Alpha2
	kfdManifest      config.KFD
	infraOutputsPath string
	tfRunner         *terraform.Runner
	dryRun           bool
}

func NewKubernetes(
	furyctlConf schema.EksclusterKfdV1Alpha2,
	kfdManifest config.KFD,
	workDir string,
	infraOutputsPath string,
	dryRun bool,
) (*Kubernetes, error) {
	kubeDir := path.Join(workDir, "kubernetes")

	phase, err := cluster.NewOperationPhase(kubeDir, kfdManifest.Tools)
	if err != nil {
		return nil, fmt.Errorf("error creating kubernetes phase: %w", err)
	}

	return &Kubernetes{
		OperationPhase:   phase,
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
		dryRun: dryRun,
	}, nil
}

func (k *Kubernetes) Exec() error {
	logrus.Info("Running kubernetes phase")

	timestamp := time.Now().Unix()

	if err := k.CreateFolder(); err != nil {
		return fmt.Errorf("error creating kubernetes phase folder: %w", err)
	}

	if err := k.copyFromTemplate(); err != nil {
		return err
	}

	if err := k.CreateFolderStructure(); err != nil {
		return fmt.Errorf("error creating kubernetes phase folder structure: %w", err)
	}

	if err := k.createTfVars(); err != nil {
		return err
	}

	if err := k.tfRunner.Init(); err != nil {
		return fmt.Errorf("error running terraform init: %w", err)
	}

	if err := k.tfRunner.Plan(timestamp); err != nil {
		return fmt.Errorf("error running terraform plan: %w", err)
	}

	if k.dryRun {
		return nil
	}

	logrus.Info("Running terraform apply...")

	out, err := k.tfRunner.Apply(timestamp)
	if err != nil {
		return fmt.Errorf("error running terraform apply: %w", err)
	}

	if err := k.createKubeconfig(out); err != nil {
		return err
	}

	if err := k.setKubeconfigEnv(); err != nil {
		return err
	}

	return k.copyKubeconfigToWorkDir()
}

func (k *Kubernetes) copyFromTemplate() error {
	var cfg template.Config

	tmpFolder, err := os.MkdirTemp("", "furyctl-kube-configs-")
	if err != nil {
		return fmt.Errorf("error creating temp folder: %w", err)
	}

	defer os.RemoveAll(tmpFolder)

	subFS, err := fs.Sub(configs.Tpl, path.Join("provisioners", "cluster", "eks"))
	if err != nil {
		return fmt.Errorf("error getting subfs: %w", err)
	}

	err = iox.CopyRecursive(subFS, tmpFolder)
	if err != nil {
		return fmt.Errorf("error copying template files: %w", err)
	}

	targetTfDir := path.Join(k.Path, "terraform")
	prefix := "kube"
	tfConfVars := map[string]map[any]any{
		"kubernetes": {
			"eks": k.kfdManifest.Kubernetes.Eks,
		},
	}

	cfg.Data = tfConfVars

	err = k.OperationPhase.CopyFromTemplate(
		cfg,
		prefix,
		tmpFolder,
		targetTfDir,
	)
	if err != nil {
		return fmt.Errorf("error generating from template files: %w", err)
	}

	return nil
}

func (k *Kubernetes) createKubeconfig(o terraform.OutputJSON) error {
	if o.Outputs["kubeconfig"] == nil {
		return errKubeconfigFromLogs
	}

	kubeString, ok := o.Outputs["kubeconfig"].Value.(string)
	if !ok {
		return errKubeconfigFromLogs
	}

	err := os.WriteFile(path.Join(k.SecretsPath, "kubeconfig"), []byte(kubeString), iox.FullRWPermAccess)
	if err != nil {
		return fmt.Errorf("error writing kubeconfig file: %w", err)
	}

	return nil
}

func (k *Kubernetes) setKubeconfigEnv() error {
	kubePath, err := filepath.Abs(path.Join(k.SecretsPath, "kubeconfig"))
	if err != nil {
		return fmt.Errorf("error getting kubeconfig absolute path: %w", err)
	}

	err = os.Setenv("KUBECONFIG", kubePath)
	if err != nil {
		return fmt.Errorf("error setting kubeconfig env: %w", err)
	}

	return nil
}

func (k *Kubernetes) copyKubeconfigToWorkDir() error {
	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("error getting current dir: %w", err)
	}

	kubePath, err := filepath.Abs(path.Join(k.SecretsPath, "kubeconfig"))
	if err != nil {
		return fmt.Errorf("error getting kubeconfig absolute path: %w", err)
	}

	kubeconfig, err := os.ReadFile(kubePath)
	if err != nil {
		return fmt.Errorf("error reading kubeconfig file: %w", err)
	}

	err = os.WriteFile(path.Join(currentDir, "kubeconfig"), kubeconfig, iox.FullRWPermAccess)
	if err != nil {
		return fmt.Errorf("error writing kubeconfig file: %w", err)
	}

	return nil
}

//nolint:gocyclo,maintidx // it will be refactored
func (k *Kubernetes) createTfVars() error {
	var buffer bytes.Buffer

	subnetIdsSource := k.furyctlConf.Spec.Kubernetes.SubnetIds
	vpcIDSource := k.furyctlConf.Spec.Kubernetes.VpcId
	allowedCidrsSource := k.furyctlConf.Spec.Kubernetes.ApiServerEndpointAccess.AllowedCidrs

	if infraOutJSON, err := os.ReadFile(path.Join(k.infraOutputsPath, "output.json")); err == nil {
		var infraOut terraform.OutputJSON

		if err := json.Unmarshal(infraOutJSON, &infraOut); err == nil {
			if infraOut.Outputs["private_subnets"] == nil {
				return errPvtSubnetNotFound
			}

			s, ok := infraOut.Outputs["private_subnets"].Value.([]interface{})
			if !ok {
				return errPvtSubnetFromOut
			}

			if infraOut.Outputs["vpc_id"] == nil {
				return ErrVpcIDNotFound
			}

			v, ok := infraOut.Outputs["vpc_id"].Value.(string)
			if !ok {
				return ErrVpcIDFromOut
			}

			if infraOut.Outputs["vpc_cidr_block"] == nil {
				return errVpcCIDRNotFound
			}

			c, ok := infraOut.Outputs["vpc_cidr_block"].Value.(string)
			if !ok {
				return errVpcCIDRFromOut
			}

			subs := make([]schema.TypesAwsSubnetId, len(s))

			for i, sub := range s {
				ss, ok := sub.(string)
				if !ok {
					return errPvtSubnetFromOut
				}

				subs[i] = schema.TypesAwsSubnetId(ss)
			}

			subnetIdsSource = subs
			vpcIDSource = schema.TypesAwsVpcId(v)
			allowedCidrsSource = []schema.TypesCidr{schema.TypesCidr(c)}
		}
	}

	_, err := buffer.WriteString(fmt.Sprintf("cluster_name = \"%v\"\n", k.furyctlConf.Metadata.Name))
	if err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	_, err = buffer.WriteString(fmt.Sprintf("cluster_version = \"%v\"\n", k.kfdManifest.Kubernetes.Eks.Version))
	if err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	_, err = buffer.WriteString(fmt.Sprintf("network = \"%v\"\n", vpcIDSource))
	if err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	subnetIds := make([]string, len(subnetIdsSource))

	for i, subnetID := range subnetIdsSource {
		subnetIds[i] = fmt.Sprintf("\"%v\"", subnetID)
	}

	_, err = buffer.WriteString(fmt.Sprintf("subnetworks = [%v]\n", strings.Join(subnetIds, ",")))
	if err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	dmzCidrRange := make([]string, len(allowedCidrsSource))

	for i, cidr := range allowedCidrsSource {
		dmzCidrRange[i] = fmt.Sprintf("\"%v\"", cidr)
	}

	_, err = buffer.WriteString(fmt.Sprintf("dmz_cidr_range = [%v]\n", strings.Join(dmzCidrRange, ",")))
	if err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	_, err = buffer.WriteString(
		fmt.Sprintf("ssh_public_key = \"%v\"\n", k.furyctlConf.Spec.Kubernetes.NodeAllowedSshPublicKey),
	)
	if err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	if k.furyctlConf.Spec.Tags != nil && len(k.furyctlConf.Spec.Tags) > 0 {
		var tags []byte

		tags, err := json.Marshal(k.furyctlConf.Spec.Tags)
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}

		_, err = buffer.WriteString(fmt.Sprintf("tags = %v\n", string(tags)))
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	}

	if len(k.furyctlConf.Spec.Kubernetes.AwsAuth.AdditionalAccounts) > 0 {
		_, err = buffer.WriteString(
			fmt.Sprintf(
				"eks_map_accounts = [\"%v\"]\n",
				strings.Join(k.furyctlConf.Spec.Kubernetes.AwsAuth.AdditionalAccounts, "\",\""),
			),
		)
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	}

	if len(k.furyctlConf.Spec.Kubernetes.AwsAuth.Users) > 0 {
		_, err = buffer.WriteString("eks_map_users = [\n")
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}

		for _, account := range k.furyctlConf.Spec.Kubernetes.AwsAuth.Users {
			_, err = buffer.WriteString(
				fmt.Sprintf(
					`{
						groups = ["%v"]
						username = "%v"
						userarn = "%v"
					},`,
					strings.Join(account.Groups, "\",\""), account.Username, account.Userarn,
				),
			)
			if err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}
		}

		_, err = buffer.WriteString("]\n")
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	}

	if len(k.furyctlConf.Spec.Kubernetes.AwsAuth.Roles) > 0 {
		_, err = buffer.WriteString("eks_map_roles = [\n")
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}

		for _, account := range k.furyctlConf.Spec.Kubernetes.AwsAuth.Roles {
			_, err = buffer.WriteString(
				fmt.Sprintf(
					`{
						groups = ["%v"]
						username = "%v"
						rolearn = "%v"
					},`,
					strings.Join(account.Groups, "\",\""), account.Username, account.Rolearn,
				),
			)
			if err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}
		}

		_, err = buffer.WriteString("]\n")
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	}

	if len(k.furyctlConf.Spec.Kubernetes.NodePools) > 0 {
		_, err = buffer.WriteString("node_pools = [\n")
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}

		for _, np := range k.furyctlConf.Spec.Kubernetes.NodePools {
			_, err = buffer.WriteString("{\n")
			if err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}

			_, err = buffer.WriteString(fmt.Sprintf("name = \"%v\"\n", np.Name))
			if err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}

			_, err = buffer.WriteString("version = null\n")
			if err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}

			_, err = buffer.WriteString(fmt.Sprintf("spot_instance = %v\n", np.Instance.Spot))
			if err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}

			_, err = buffer.WriteString(fmt.Sprintf("min_size = %v\n", np.Size.Min))
			if err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}

			_, err = buffer.WriteString(fmt.Sprintf("max_size = %v\n", np.Size.Max))
			if err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}

			_, err = buffer.WriteString(fmt.Sprintf("instance_type = \"%v\"\n", np.Instance.Type))
			if err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}

			if len(np.AttachedTargetGroups) > 0 {
				attachedTargetGroups := make([]string, len(np.AttachedTargetGroups))

				for i, tg := range np.AttachedTargetGroups {
					attachedTargetGroups[i] = fmt.Sprintf("\"%v\"", tg)
				}

				_, err = buffer.WriteString(
					fmt.Sprintf(
						"eks_target_group_arns = [%v]\n",
						strings.Join(attachedTargetGroups, ","),
					))
				if err != nil {
					return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
				}
			}

			_, err = buffer.WriteString(fmt.Sprintf("volume_size = %v\n", np.Instance.VolumeSize))
			if err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}

			if len(np.AdditionalFirewallRules) > 0 {
				_, err = buffer.WriteString("additional_firewall_rules = [\n")
				if err != nil {
					return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
				}

				for _, fwRule := range np.AdditionalFirewallRules {
					fwRuleTags := "{}"

					if len(fwRule.Tags) > 0 {
						var tags []byte

						tags, err := json.Marshal(fwRule.Tags)
						if err != nil {
							return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
						}

						fwRuleTags = string(tags)
					}

					_, err = buffer.WriteString(
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
					if err != nil {
						return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
					}
				}

				_, err = buffer.WriteString("]\n")
				if err != nil {
					return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
				}
			} else {
				_, err = buffer.WriteString("additional_firewall_rules = []\n")
				if err != nil {
					return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
				}
			}

			if len(np.SubnetIds) > 0 {
				npSubNetIds := make([]string, len(np.SubnetIds))

				for i, subnetID := range np.SubnetIds {
					npSubNetIds[i] = fmt.Sprintf("\"%v\"", subnetID)
				}

				_, err = buffer.WriteString(fmt.Sprintf("subnetworks = [%v]\n", strings.Join(npSubNetIds, ",")))
				if err != nil {
					return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
				}
			} else {
				_, err = buffer.WriteString("subnetworks = null\n")
				if err != nil {
					return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
				}
			}

			if len(np.Labels) > 0 {
				var labels []byte

				labels, err := json.Marshal(np.Labels)
				if err != nil {
					return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
				}

				_, err = buffer.WriteString(fmt.Sprintf("labels = %v\n", string(labels)))
				if err != nil {
					return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
				}
			} else {
				_, err = buffer.WriteString("labels = {}\n")
				if err != nil {
					return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
				}
			}

			if len(np.Taints) > 0 {
				_, err = buffer.WriteString(fmt.Sprintf("taints = [\"%v\"]\n", strings.Join(np.Taints, "\",\"")))
				if err != nil {
					return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
				}
			} else {
				_, err = buffer.WriteString("taints = []\n")
				if err != nil {
					return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
				}
			}

			if len(np.Tags) > 0 {
				var tags []byte

				tags, err := json.Marshal(np.Tags)
				if err != nil {
					return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
				}

				_, err = buffer.WriteString(fmt.Sprintf("tags = %v\n", string(tags)))
				if err != nil {
					return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
				}
			} else {
				_, err = buffer.WriteString("tags = {}\n")
				if err != nil {
					return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
				}
			}

			_, err = buffer.WriteString("},\n")
			if err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}
		}

		_, err = buffer.WriteString("]\n")
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	}

	targetTfVars := path.Join(k.Path, "terraform", "main.auto.tfvars")

	err = os.WriteFile(targetTfVars, buffer.Bytes(), iox.FullRWPermAccess)
	if err != nil {
		return fmt.Errorf("error writing terraform vars file: %w", err)
	}

	return nil
}
