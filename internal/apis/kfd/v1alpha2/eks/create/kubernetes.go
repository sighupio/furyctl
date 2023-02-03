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
	"net"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/config"
	"github.com/sighupio/fury-distribution/pkg/schema"
	"github.com/sighupio/furyctl/configs"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/template"
	"github.com/sighupio/furyctl/internal/tool/awscli"
	"github.com/sighupio/furyctl/internal/tool/terraform"
	bytesx "github.com/sighupio/furyctl/internal/x/bytes"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
	kubex "github.com/sighupio/furyctl/internal/x/kube"
	netx "github.com/sighupio/furyctl/internal/x/net"
)

var (
	errKubeconfigFromLogs  = errors.New("can't get kubeconfig from logs")
	errPvtSubnetNotFound   = errors.New("private_subnets not found in infra output")
	errPvtSubnetFromOut    = errors.New("cannot read private_subnets from infrastructure's output.json")
	errVpcCIDRFromOut      = errors.New("cannot read vpc_cidr_block from infrastructure's output.json")
	errVpcCIDRNotFound     = errors.New("vpc_cidr_block not found in infra output")
	errVpcIDNotFound       = errors.New("vpc id not found: you forgot to specify one or the infrastructure phase failed")
	errParsingCIDR         = errors.New("error parsing CIDR")
	errResolvingDNS        = errors.New("error resolving DNS record")
	errVpcIDNotProvided    = errors.New("vpc_id not provided")
	errCIDRBlockFromVpc    = errors.New("error getting CIDR block from VPC")
	errKubeAPIUnreachable  = errors.New("kubernetes API is not reachable")
	errAddingOffsetToIPNet = errors.New("error adding offset to ipnet")
)

const (
	nodePoolDefaultVolumeSize = 35
	// https://docs.aws.amazon.com/vpc/latest/userguide/vpc-dns.html
	awsDNSServerIPOffset = 2
)

type Kubernetes struct {
	*cluster.OperationPhase
	furyctlConf      schema.EksclusterKfdV1Alpha2
	kfdManifest      config.KFD
	infraOutputsPath string
	tfRunner         *terraform.Runner
	awsRunner        *awscli.Runner
	dryRun           bool
}

func NewKubernetes(
	furyctlConf schema.EksclusterKfdV1Alpha2,
	kfdManifest config.KFD,
	infraOutputsPath string,
	paths cluster.CreatorPaths,
	dryRun bool,
) (*Kubernetes, error) {
	kubeDir := path.Join(paths.WorkDir, cluster.OperationPhaseKubernetes)

	phase, err := cluster.NewOperationPhase(kubeDir, kfdManifest.Tools, paths.BinPath)
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
		awsRunner: awscli.NewRunner(
			execx.NewStdExecutor(),
			awscli.Paths{
				Awscli:  "aws",
				WorkDir: phase.Path,
			},
		),
		dryRun: dryRun,
	}, nil
}

func (k *Kubernetes) Exec() error {
	timestamp := time.Now().Unix()

	logrus.Info("Creating Kubernetes Fury cluster...")

	logrus.Debug("Create: running kubernetes phase...")

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

	logrus.Info("Checking connection to the VPC...")

	if err := k.checkVPCConnection(); err != nil {
		logrus.Debugf("error checking VPC connection: %v", err)

		if k.furyctlConf.Spec.Infrastructure != nil {
			if k.furyctlConf.Spec.Infrastructure.Vpc.Vpn != nil {
				return fmt.Errorf("%w please check your VPN connection and try again", errKubeAPIUnreachable)
			}
		}

		return fmt.Errorf("%w please check your VPC configuration and try again", errKubeAPIUnreachable)
	}

	logrus.Info("Creating cloud resources, this could take a while...")

	out, err := k.tfRunner.Apply(timestamp)
	if err != nil {
		return fmt.Errorf("cannot create cloud resources: %w", err)
	}

	if out.Outputs["kubeconfig"] == nil {
		return errKubeconfigFromLogs
	}

	kubeString, ok := out.Outputs["kubeconfig"].Value.(string)
	if !ok {
		return errKubeconfigFromLogs
	}

	p, err := kubex.CreateConfig([]byte(kubeString), k.SecretsPath)
	if err != nil {
		return fmt.Errorf("error creating kubeconfig: %w", err)
	}

	if err := kubex.SetConfigEnv(p); err != nil {
		return fmt.Errorf("error setting kubeconfig env: %w", err)
	}

	if err := kubex.CopyConfigToWorkDir(p); err != nil {
		return fmt.Errorf("error copying kubeconfig: %w", err)
	}

	return nil
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
		"terraform": {
			"backend": map[string]any{
				"s3": map[string]any{
					"bucketName": k.furyctlConf.Spec.ToolsConfiguration.Terraform.State.S3.BucketName,
					"keyPrefix":  k.furyctlConf.Spec.ToolsConfiguration.Terraform.State.S3.KeyPrefix,
					"region":     k.furyctlConf.Spec.ToolsConfiguration.Terraform.State.S3.Region,
				},
			},
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

//nolint:gocyclo,maintidx,funlen // it will be refactored
func (k *Kubernetes) createTfVars() error {
	var buffer bytes.Buffer

	var allowedCidrsSource []schema.TypesCidr

	subnetIdsSource := k.furyctlConf.Spec.Kubernetes.SubnetIds
	vpcIDSource := k.furyctlConf.Spec.Kubernetes.VpcId

	if k.furyctlConf.Spec.Kubernetes.ApiServerEndpointAccess != nil {
		allowedCidrsSource = k.furyctlConf.Spec.Kubernetes.ApiServerEndpointAccess.AllowedCidrs
	}

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
			vpcID := schema.TypesAwsVpcId(v)
			vpcIDSource = &vpcID
			allowedCidrsSource = []schema.TypesCidr{schema.TypesCidr(c)}
		}
	}

	err := bytesx.SafeWriteToBuffer(
		&buffer,
		"cluster_name = \"%v\"\n",
		k.furyctlConf.Metadata.Name,
	)
	if err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	err = bytesx.SafeWriteToBuffer(
		&buffer,
		"cluster_version = \"%v\"\n",
		k.kfdManifest.Kubernetes.Eks.Version,
	)
	if err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	err = bytesx.SafeWriteToBuffer(
		&buffer,
		"node_pools_launch_kind = \"%v\"\n",
		k.furyctlConf.Spec.Kubernetes.NodePoolsLaunchKind,
	)
	if err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	if k.furyctlConf.Spec.Kubernetes.LogRetentionDays != nil {
		err = bytesx.SafeWriteToBuffer(
			&buffer,
			"cluster_log_retention_days = %v\n",
			*k.furyctlConf.Spec.Kubernetes.LogRetentionDays,
		)
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	}

	if vpcIDSource == nil {
		if !k.dryRun {
			return errVpcIDNotFound
		}

		vpcIDSource = new(schema.TypesAwsVpcId)
	}

	err = bytesx.SafeWriteToBuffer(
		&buffer,
		"network = \"%v\"\n",
		*vpcIDSource,
	)
	if err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	subnetIds := make([]string, len(subnetIdsSource))

	for i, subnetID := range subnetIdsSource {
		subnetIds[i] = fmt.Sprintf("\"%v\"", subnetID)
	}

	err = bytesx.SafeWriteToBuffer(
		&buffer,
		"subnetworks = [%v]\n",
		strings.Join(subnetIds, ","),
	)
	if err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	dmzCidrRange := make([]string, len(allowedCidrsSource))

	for i, cidr := range allowedCidrsSource {
		dmzCidrRange[i] = fmt.Sprintf("\"%v\"", cidr)
	}

	err = bytesx.SafeWriteToBuffer(
		&buffer,
		"dmz_cidr_range = [%v]\n",
		strings.Join(dmzCidrRange, ","),
	)
	if err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	err = bytesx.SafeWriteToBuffer(
		&buffer,
		"ssh_public_key = \"%v\"\n",
		k.furyctlConf.Spec.Kubernetes.NodeAllowedSshPublicKey,
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

		err = bytesx.SafeWriteToBuffer(
			&buffer,
			"tags = %v\n",
			string(tags),
		)
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	}

	err = k.addAwsAuthToTfVars(&buffer)
	if err != nil {
		return fmt.Errorf("error writing AWS Auth to Terraform vars file: %w", err)
	}

	if len(k.furyctlConf.Spec.Kubernetes.NodePools) > 0 {
		err = bytesx.SafeWriteToBuffer(
			&buffer,
			"node_pools = [\n",
		)
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}

		for _, np := range k.furyctlConf.Spec.Kubernetes.NodePools {
			err = bytesx.SafeWriteToBuffer(
				&buffer,
				"{\n",
			)
			if err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}

			err = bytesx.SafeWriteToBuffer(
				&buffer,
				"name = \"%v\"\n",
				np.Name,
			)
			if err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}

			err = bytesx.SafeWriteToBuffer(
				&buffer,
				"version = null\n",
			)
			if err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}

			spot := "false"

			if np.Instance.Spot != nil {
				spot = strconv.FormatBool(*np.Instance.Spot)
			}

			err = bytesx.SafeWriteToBuffer(
				&buffer,
				"spot_instance = %v\n",
				spot,
			)
			if err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}

			err = bytesx.SafeWriteToBuffer(
				&buffer,
				"min_size = %v\n",
				np.Size.Min,
			)
			if err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}

			err = bytesx.SafeWriteToBuffer(
				&buffer,
				"max_size = %v\n",
				np.Size.Max,
			)
			if err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}

			err = bytesx.SafeWriteToBuffer(
				&buffer,
				"instance_type = \"%v\"\n",
				np.Instance.Type,
			)
			if err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}

			if len(np.AttachedTargetGroups) > 0 {
				attachedTargetGroups := make([]string, len(np.AttachedTargetGroups))

				for i, tg := range np.AttachedTargetGroups {
					attachedTargetGroups[i] = fmt.Sprintf("\"%v\"", tg)
				}

				err = bytesx.SafeWriteToBuffer(
					&buffer,
					"eks_target_group_arns = [%v]\n",
					strings.Join(attachedTargetGroups, ","),
				)
				if err != nil {
					return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
				}
			}

			volumeSize := nodePoolDefaultVolumeSize

			if np.Instance.VolumeSize != nil {
				volumeSize = *np.Instance.VolumeSize
			}

			err = bytesx.SafeWriteToBuffer(
				&buffer,
				"volume_size = %v\n",
				volumeSize,
			)
			if err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}

			err = k.addFirewallRulesToNodePool(&buffer, np)
			if err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}

			if len(np.SubnetIds) > 0 {
				npSubNetIds := make([]string, len(np.SubnetIds))

				for i, subnetID := range np.SubnetIds {
					npSubNetIds[i] = fmt.Sprintf("\"%v\"", subnetID)
				}

				err = bytesx.SafeWriteToBuffer(
					&buffer,
					"subnetworks = [%v]\n",
					strings.Join(npSubNetIds, ","),
				)
				if err != nil {
					return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
				}
			} else {
				err = bytesx.SafeWriteToBuffer(
					&buffer,
					"subnetworks = null\n",
				)
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

				err = bytesx.SafeWriteToBuffer(
					&buffer,
					"labels = %v\n",
					string(labels),
				)
				if err != nil {
					return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
				}
			} else {
				err = bytesx.SafeWriteToBuffer(
					&buffer,
					"labels = {}\n",
				)
				if err != nil {
					return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
				}
			}

			if len(np.Taints) > 0 {
				err = bytesx.SafeWriteToBuffer(
					&buffer,
					"taints = [\"%v\"]\n",
					strings.Join(np.Taints, "\",\""),
				)
				if err != nil {
					return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
				}
			} else {
				err = bytesx.SafeWriteToBuffer(
					&buffer,
					"taints = []\n",
				)
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

				err = bytesx.SafeWriteToBuffer(
					&buffer,
					"tags = %v\n",
					string(tags),
				)
				if err != nil {
					return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
				}
			} else {
				err = bytesx.SafeWriteToBuffer(
					&buffer,
					"tags = {}\n",
				)
				if err != nil {
					return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
				}
			}

			err = bytesx.SafeWriteToBuffer(
				&buffer,
				"},\n",
			)
			if err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}
		}

		err = bytesx.SafeWriteToBuffer(
			&buffer,
			"]\n",
		)
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

func (k *Kubernetes) addAwsAuthToTfVars(buffer *bytes.Buffer) error {
	var err error

	if k.furyctlConf.Spec.Kubernetes.AwsAuth != nil {
		if len(k.furyctlConf.Spec.Kubernetes.AwsAuth.AdditionalAccounts) > 0 {
			err = bytesx.SafeWriteToBuffer(
				buffer,
				"eks_map_accounts = [\"%v\"]\n",
				strings.Join(k.furyctlConf.Spec.Kubernetes.AwsAuth.AdditionalAccounts, "\",\""),
			)
			if err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}
		}

		if len(k.furyctlConf.Spec.Kubernetes.AwsAuth.Users) > 0 {
			err = bytesx.SafeWriteToBuffer(
				buffer,
				"eks_map_users = [\n",
			)
			if err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}

			for i, account := range k.furyctlConf.Spec.Kubernetes.AwsAuth.Users {
				content := "{\ngroups = [\"%v\"]\nusername = \"%v\"\nuserarn = \"%v\"}"

				if i < len(k.furyctlConf.Spec.Kubernetes.AwsAuth.Users)-1 {
					content += ","
				}

				err = bytesx.SafeWriteToBuffer(
					buffer,
					content,
					strings.Join(account.Groups, "\",\""),
					account.Username,
					account.Userarn,
				)
				if err != nil {
					return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
				}
			}

			err = bytesx.SafeWriteToBuffer(
				buffer,
				"]\n",
			)
			if err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}
		}

		if len(k.furyctlConf.Spec.Kubernetes.AwsAuth.Roles) > 0 {
			err = bytesx.SafeWriteToBuffer(
				buffer,
				"eks_map_roles = [\n",
			)
			if err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}

			for i, account := range k.furyctlConf.Spec.Kubernetes.AwsAuth.Roles {
				content := "{\ngroups = [\"%v\"]\nusername = \"%v\"\nrolearn = \"%v\"}"

				if i < len(k.furyctlConf.Spec.Kubernetes.AwsAuth.Roles)-1 {
					content += ","
				}

				err = bytesx.SafeWriteToBuffer(
					buffer,
					content,
					strings.Join(account.Groups, "\",\""),
					account.Username,
					account.Rolearn,
				)
				if err != nil {
					return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
				}
			}

			err = bytesx.SafeWriteToBuffer(
				buffer,
				"]\n",
			)
			if err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}
		}
	}

	return nil
}

func (*Kubernetes) addFirewallRulesToNodePool(buffer *bytes.Buffer, np schema.SpecKubernetesNodePool) error {
	if len(np.AdditionalFirewallRules) > 0 {
		err := bytesx.SafeWriteToBuffer(
			buffer,
			"additional_firewall_rules = [\n",
		)
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}

		for i, fwRule := range np.AdditionalFirewallRules {
			fwRuleTags := "{}"

			if len(fwRule.Tags) > 0 {
				var tags []byte

				tags, err := json.Marshal(fwRule.Tags)
				if err != nil {
					return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
				}

				fwRuleTags = string(tags)
			}

			content := "{\nname = \"%v\"\ndirection = \"%v\"\ncidr_block = %v\nprotocol = \"%v\"\n" +
				"ports = \"%v\"\ntags = %v\n}"

			if i < len(np.AdditionalFirewallRules)-1 {
				content += ","
			}

			dmzCidrRanges := make([]string, len(fwRule.CidrBlocks))

			for i, cidr := range fwRule.CidrBlocks {
				dmzCidrRanges[i] = fmt.Sprintf("\"%v\"", cidr)
			}

			cidrRange := ""

			if len(dmzCidrRanges) > 0 {
				cidrRange = dmzCidrRanges[0]
			}

			err = bytesx.SafeWriteToBuffer(
				buffer,
				content,
				fwRule.Name,
				fwRule.Type,
				cidrRange,
				fwRule.Protocol,
				fmt.Sprintf("%v-%v", fwRule.Ports.From, fwRule.Ports.To),
				fwRuleTags,
			)
			if err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}
		}

		err = bytesx.SafeWriteToBuffer(
			buffer,
			"]\n",
		)
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	} else {
		err := bytesx.SafeWriteToBuffer(
			buffer,
			"additional_firewall_rules = []\n",
		)
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	}

	return nil
}

func (k *Kubernetes) checkVPCConnection() error {
	var (
		cidr string
		err  error
	)

	if k.furyctlConf.Spec.Infrastructure != nil {
		cidr = string(k.furyctlConf.Spec.Infrastructure.Vpc.Network.Cidr)
	} else {
		vpcID := k.furyctlConf.Spec.Kubernetes.VpcId
		if vpcID == nil {
			return errVpcIDNotProvided
		}

		cidr, err = k.awsRunner.Ec2(
			"describe-vpcs",
			"--vpc-ids",
			string(*vpcID),
			"--query",
			"Vpcs[0].CidrBlock",
			"--output",
			"text",
		)
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, errCIDRBlockFromVpc, err)
		}
	}

	return k.queryAWSDNSServer(cidr)
}

func (*Kubernetes) queryAWSDNSServer(cidr string) error {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf(SErrWrapWithStr, errParsingCIDR, err)
	}

	offIPNet, err := netx.AddOffsetToIPNet(ipNet, awsDNSServerIPOffset)
	if err != nil {
		return fmt.Errorf(SErrWrapWithStr, errAddingOffsetToIPNet, err)
	}

	err = netx.DNSQuery(offIPNet.IP.String(), "google.com.")
	if err != nil {
		return fmt.Errorf(SErrWrapWithStr, errResolvingDNS, err)
	}

	return nil
}
