// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/fury-distribution/pkg/apis/ekscluster/v1alpha2/private"
	"github.com/sighupio/furyctl/configs"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/merge"
	"github.com/sighupio/furyctl/internal/template"
	"github.com/sighupio/furyctl/internal/tool/terraform"
	bytesx "github.com/sighupio/furyctl/internal/x/bytes"
	iox "github.com/sighupio/furyctl/internal/x/io"
	"github.com/sighupio/furyctl/internal/x/slices"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

const (
	nodePoolDefaultVolumeSize = 35
)

type Kubernetes struct {
	*cluster.OperationPhase

	FuryctlConf                        private.EksclusterKfdV1Alpha2
	FuryctlConfPath                    string
	DistroPath                         string
	KFDManifest                        config.KFD
	DryRun                             bool
	InfrastructureTerraformOutputsPath string
}

func (k *Kubernetes) Prepare() error {
	if err := k.CreateRootFolder(); err != nil {
		return fmt.Errorf("error creating kubernetes phase folder: %w", err)
	}

	cfg, err := k.mergeConfig()
	if err != nil {
		return fmt.Errorf("error merging furyctl configuration: %w", err)
	}

	if err := k.copyFromTemplate(cfg); err != nil {
		return err
	}

	if err := k.CreateTerraformFolderStructure(); err != nil {
		return fmt.Errorf("error creating kubernetes phase folder structure: %w", err)
	}

	return k.createTfVars()
}

func (k *Kubernetes) mergeConfig() (template.Config, error) {
	var cfg template.Config

	defaultsFilePath := path.Join(k.DistroPath, "defaults", "ekscluster-kfd-v1alpha2.yaml")

	defaultsFile, err := yamlx.FromFileV2[map[any]any](defaultsFilePath)
	if err != nil {
		return cfg, fmt.Errorf("%s - %w", defaultsFilePath, err)
	}

	furyctlConf, err := yamlx.FromFileV2[map[any]any](k.FuryctlConfPath)
	if err != nil {
		return cfg, fmt.Errorf("%s - %w", k.FuryctlConfPath, err)
	}

	merger := merge.NewMerger(
		merge.NewDefaultModel(defaultsFile, ".data"),
		merge.NewDefaultModel(furyctlConf, ".spec.distribution"),
	)

	_, err = merger.Merge()
	if err != nil {
		return cfg, fmt.Errorf("error merging files: %w", err)
	}

	reverseMerger := merge.NewMerger(
		*merger.GetCustom(),
		*merger.GetBase(),
	)

	_, err = reverseMerger.Merge()
	if err != nil {
		return cfg, fmt.Errorf("error merging files: %w", err)
	}

	cfg, err = template.NewConfig(reverseMerger, reverseMerger, []string{"terraform", ".gitignore"})
	if err != nil {
		return cfg, fmt.Errorf("error creating template config: %w", err)
	}

	return cfg, nil
}

func (k *Kubernetes) copyFromTemplate(furyctlCfg template.Config) error {
	var cfg template.Config

	tmpFolder, err := os.MkdirTemp("", "furyctl-kubernetes-configs-")
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

	eksInstallerPath := path.Join(k.Path, "..", "vendor", "installers", "eks", "modules", "eks")

	nodeSelector, tolerations, err := k.getCommonDataFromDistribution(furyctlCfg)
	if err != nil {
		return err
	}

	tfConfVars := map[string]map[any]any{
		"spec": {
			"region": k.FuryctlConf.Spec.Region,
			"tags":   k.FuryctlConf.Spec.Tags,
		},
		"kubernetes": {
			"installerPath": eksInstallerPath,
			"tfVersion":     k.KFDManifest.Tools.Common.Terraform.Version,
		},
		"distribution": {
			"nodeSelector": nodeSelector,
			"tolerations":  tolerations,
		},
		"terraform": {
			"backend": map[string]any{
				"s3": map[string]any{
					"bucketName":           k.FuryctlConf.Spec.ToolsConfiguration.Terraform.State.S3.BucketName,
					"keyPrefix":            k.FuryctlConf.Spec.ToolsConfiguration.Terraform.State.S3.KeyPrefix,
					"region":               k.FuryctlConf.Spec.ToolsConfiguration.Terraform.State.S3.Region,
					"skipRegionValidation": k.FuryctlConf.Spec.ToolsConfiguration.Terraform.State.S3.SkipRegionValidation,
				},
			},
		},
		"options": {
			"dryRun": k.DryRun,
		},
	}

	cfg.Data = tfConfVars

	err = k.CopyFromTemplate(
		cfg,
		"kubernetes",
		tmpFolder,
		targetTfDir,
		k.FuryctlConfPath,
	)
	if err != nil {
		return fmt.Errorf("error generating from template files: %w", err)
	}

	return nil
}

func (*Kubernetes) getCommonDataFromDistribution(furyctlCfg template.Config) (map[any]any, []any, error) {
	var nodeSelector map[any]any

	var tolerations []any

	var ok bool

	model := merge.NewDefaultModel(furyctlCfg.Data["spec"], ".distribution.common")

	commonData, err := model.Get()
	if err != nil {
		return nodeSelector, tolerations, fmt.Errorf("error getting common data from distribution: %w", err)
	}

	if commonData["nodeSelector"] != nil {
		nodeSelector, ok = commonData["nodeSelector"].(map[any]any)
		if !ok {
			return nodeSelector, tolerations, fmt.Errorf("error getting nodeSelector from distribution: %w", err)
		}
	}

	if commonData["tolerations"] != nil {
		tolerations, ok = commonData["tolerations"].([]any)
		if !ok {
			return nodeSelector, tolerations, fmt.Errorf("error getting tolerations from distribution: %w", err)
		}
	}

	return nodeSelector, tolerations, nil
}

//nolint:gocyclo,maintidx // it will be refactored
func (k *Kubernetes) createTfVars() error {
	var buffer bytes.Buffer

	subnetIdsSource := k.FuryctlConf.Spec.Kubernetes.SubnetIds
	vpcIDSource := k.FuryctlConf.Spec.Kubernetes.VpcId

	allowedClusterEndpointPrivateAccessCIDRs := k.FuryctlConf.Spec.Kubernetes.ApiServer.PrivateAccessCidrs
	allowedClusterEndpointPublicAccessCIDRs := k.FuryctlConf.Spec.Kubernetes.ApiServer.PublicAccessCidrs

	if k.FuryctlConf.Spec.Infrastructure != nil &&
		k.FuryctlConf.Spec.Infrastructure.Vpc != nil {
		if infraOutJSON, err := os.ReadFile(path.Join(k.InfrastructureTerraformOutputsPath, "output.json")); err == nil {
			var infraOut terraform.OutputJSON

			if err := json.Unmarshal(infraOutJSON, &infraOut); err == nil {
				if infraOut["private_subnets"] == nil {
					return ErrPvtSubnetNotFound
				}

				s, ok := infraOut["private_subnets"].Value.([]any)
				if !ok {
					return ErrPvtSubnetFromOut
				}

				if infraOut["vpc_id"] == nil {
					return ErrVpcIDNotFound
				}

				v, ok := infraOut["vpc_id"].Value.(string)
				if !ok {
					return ErrVpcIDFromOut
				}

				if infraOut["vpc_cidr_block"] == nil {
					return ErrVpcCIDRNotFound
				}

				c, ok := infraOut["vpc_cidr_block"].Value.(string)
				if !ok {
					return ErrVpcCIDRFromOut
				}

				subs := make([]private.TypesAwsSubnetId, len(s))

				for i, sub := range s {
					ss, ok := sub.(string)
					if !ok {
						return ErrPvtSubnetFromOut
					}

					subs[i] = private.TypesAwsSubnetId(ss)
				}

				subnetIdsSource = subs
				vpcID := private.TypesAwsVpcId(v)
				vpcIDSource = &vpcID

				allowedClusterEndpointPrivateAccessCIDRs = append(
					allowedClusterEndpointPrivateAccessCIDRs,
					private.TypesCidr(c),
				)
			}
		}
	}

	allowedClusterEndpointPrivateAccessCIDRs = slices.Uniq(allowedClusterEndpointPrivateAccessCIDRs)

	if err := bytesx.SafeWriteToBuffer(
		&buffer,
		"cluster_name = \"%v\"\n",
		filepath.Dir(k.FuryctlConfPath),
		k.FuryctlConf.Metadata.Name,
	); err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	if err := bytesx.SafeWriteToBuffer(
		&buffer,
		"kubectl_path = \"%s\"\n",
		filepath.Dir(k.FuryctlConfPath),
		k.KubectlPath,
	); err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	if err := bytesx.SafeWriteToBuffer(
		&buffer,
		"cluster_version = \"%v\"\n",
		filepath.Dir(k.FuryctlConfPath),
		k.KFDManifest.Kubernetes.Eks.Version,
	); err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	if err := bytesx.SafeWriteToBuffer(
		&buffer,
		"cluster_endpoint_private_access = %v\n",
		filepath.Dir(k.FuryctlConfPath),
		k.FuryctlConf.Spec.Kubernetes.ApiServer.PrivateAccess,
	); err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	clusterEndpointPrivateAccessCidrs := make([]string, len(allowedClusterEndpointPrivateAccessCIDRs))

	for i, cidr := range allowedClusterEndpointPrivateAccessCIDRs {
		clusterEndpointPrivateAccessCidrs[i] = fmt.Sprintf("\"%v\"", cidr)
	}

	if err := bytesx.SafeWriteToBuffer(
		&buffer,
		"cluster_endpoint_private_access_cidrs = [%v]\n",
		filepath.Dir(k.FuryctlConfPath),
		strings.Join(clusterEndpointPrivateAccessCidrs, ","),
	); err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	if err := bytesx.SafeWriteToBuffer(
		&buffer,
		"cluster_endpoint_public_access = %v\n",
		filepath.Dir(k.FuryctlConfPath),
		k.FuryctlConf.Spec.Kubernetes.ApiServer.PublicAccess,
	); err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	if k.FuryctlConf.Spec.Kubernetes.ApiServer.PublicAccess && len(allowedClusterEndpointPublicAccessCIDRs) == 0 {
		allowedClusterEndpointPublicAccessCIDRs = append(
			allowedClusterEndpointPublicAccessCIDRs,
			private.TypesCidr("0.0.0.0/0"),
		)
	}

	clusterEndpointPublicAccessCidrs := make([]string, len(allowedClusterEndpointPublicAccessCIDRs))

	for i, cidr := range allowedClusterEndpointPublicAccessCIDRs {
		clusterEndpointPublicAccessCidrs[i] = fmt.Sprintf("\"%v\"", cidr)
	}

	if err := bytesx.SafeWriteToBuffer(
		&buffer,
		"cluster_endpoint_public_access_cidrs = [%v]\n",
		filepath.Dir(k.FuryctlConfPath),
		strings.Join(clusterEndpointPublicAccessCidrs, ","),
	); err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	if k.FuryctlConf.Spec.Kubernetes.ServiceIpV4Cidr == nil {
		if err := bytesx.SafeWriteToBuffer(
			&buffer,
			"cluster_service_ipv4_cidr = null\n",
			filepath.Dir(k.FuryctlConfPath),
		); err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	} else {
		if err := bytesx.SafeWriteToBuffer(
			&buffer,
			"cluster_service_ipv4_cidr = \"%v\"\n",
			filepath.Dir(k.FuryctlConfPath),
			k.FuryctlConf.Spec.Kubernetes.ServiceIpV4Cidr,
		); err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	}

	if err := bytesx.SafeWriteToBuffer(
		&buffer,
		"node_pools_launch_kind = \"%v\"\n",
		filepath.Dir(k.FuryctlConfPath),
		k.FuryctlConf.Spec.Kubernetes.NodePoolsLaunchKind,
	); err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	if k.FuryctlConf.Spec.Kubernetes.LogRetentionDays != nil {
		if err := bytesx.SafeWriteToBuffer(
			&buffer,
			"cluster_log_retention_days = %v\n",
			filepath.Dir(k.FuryctlConfPath),
			*k.FuryctlConf.Spec.Kubernetes.LogRetentionDays,
		); err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	}

	if vpcIDSource == nil {
		if !k.DryRun {
			return ErrVpcIDNotFound
		}

		vpcIDSource = new(private.TypesAwsVpcId)
	}

	if err := bytesx.SafeWriteToBuffer(
		&buffer,
		"vpc_id = \"%v\"\n",
		filepath.Dir(k.FuryctlConfPath),
		*vpcIDSource,
	); err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	subnetIds := make([]string, len(subnetIdsSource))

	for i, subnetID := range subnetIdsSource {
		subnetIds[i] = fmt.Sprintf("\"%v\"", subnetID)
	}

	if err := bytesx.SafeWriteToBuffer(
		&buffer,
		"subnets = [%v]\n",
		filepath.Dir(k.FuryctlConfPath),
		strings.Join(subnetIds, ","),
	); err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	if err := bytesx.SafeWriteToBuffer(
		&buffer,
		"ssh_public_key = \"%v\"\n",
		filepath.Dir(k.FuryctlConfPath),
		k.FuryctlConf.Spec.Kubernetes.NodeAllowedSshPublicKey,
	); err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	if err := k.addAwsAuthToTfVars(&buffer); err != nil {
		return fmt.Errorf("error writing AWS Auth to Terraform vars file: %w", err)
	}

	if len(k.FuryctlConf.Spec.Kubernetes.NodePools) > 0 {
		if err := k.addNodePoolsToTfVars(&buffer); err != nil {
			return fmt.Errorf("error writing node pools to Terraform vars file: %w", err)
		}
	}

	targetTfVars := path.Join(k.Path, "terraform", "main.auto.tfvars")

	if err := os.WriteFile(targetTfVars, buffer.Bytes(), iox.FullRWPermAccess); err != nil {
		return fmt.Errorf("error writing terraform vars file: %w", err)
	}

	return nil
}

func (k *Kubernetes) addAwsAuthToTfVars(buffer *bytes.Buffer) error {
	var err error

	if k.FuryctlConf.Spec.Kubernetes.AwsAuth != nil {
		if len(k.FuryctlConf.Spec.Kubernetes.AwsAuth.AdditionalAccounts) > 0 {
			err = bytesx.SafeWriteToBuffer(
				buffer,
				"eks_map_accounts = [\"%v\"]\n",
				filepath.Dir(k.FuryctlConfPath),
				strings.Join(k.FuryctlConf.Spec.Kubernetes.AwsAuth.AdditionalAccounts, "\",\""),
			)
			if err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}
		}

		if len(k.FuryctlConf.Spec.Kubernetes.AwsAuth.Users) > 0 {
			err = k.addAwsAuthUsers(buffer)
			if err != nil {
				return fmt.Errorf("error writing AWS Auth to Terraform vars file: %w", err)
			}
		}

		if len(k.FuryctlConf.Spec.Kubernetes.AwsAuth.Roles) > 0 {
			err = k.addAwsAuthRoles(buffer)
			if err != nil {
				return fmt.Errorf("error writing AWS Auth to Terraform vars file: %w", err)
			}
		}
	}

	return nil
}

//nolint:dupl // types are different, it's not a duplicate
func (k *Kubernetes) addAwsAuthUsers(buffer *bytes.Buffer) error {
	err := bytesx.SafeWriteToBuffer(
		buffer,
		"eks_map_users = [\n",
		filepath.Dir(k.FuryctlConfPath),
	)
	if err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	for i, account := range k.FuryctlConf.Spec.Kubernetes.AwsAuth.Users {
		content := "{\ngroups = [\"%v\"]\nusername = \"%v\"\nuserarn = \"%v\"}"

		if i < len(k.FuryctlConf.Spec.Kubernetes.AwsAuth.Users)-1 {
			content += ","
		}

		err = bytesx.SafeWriteToBuffer(
			buffer,
			content,
			filepath.Dir(k.FuryctlConfPath),
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
		filepath.Dir(k.FuryctlConfPath),
	)
	if err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	return nil
}

//nolint:dupl // types are different, it's not a duplicate
func (k *Kubernetes) addAwsAuthRoles(buffer *bytes.Buffer) error {
	err := bytesx.SafeWriteToBuffer(
		buffer,
		"eks_map_roles = [\n",
		filepath.Dir(k.FuryctlConfPath),
	)
	if err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	for i, account := range k.FuryctlConf.Spec.Kubernetes.AwsAuth.Roles {
		content := "{\ngroups = [\"%v\"]\nusername = \"%v\"\nrolearn = \"%v\"}"

		if i < len(k.FuryctlConf.Spec.Kubernetes.AwsAuth.Roles)-1 {
			content += ","
		}

		err = bytesx.SafeWriteToBuffer(
			buffer,
			content,
			filepath.Dir(k.FuryctlConfPath),
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
		filepath.Dir(k.FuryctlConfPath),
	)
	if err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	return nil
}

func (k *Kubernetes) addNodePoolsToTfVars(buffer *bytes.Buffer) error {
	if err := bytesx.SafeWriteToBuffer(buffer, "node_pools = [\n", filepath.Dir(k.FuryctlConfPath)); err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	for _, np := range k.FuryctlConf.Spec.Kubernetes.NodePools {
		if err := bytesx.SafeWriteToBuffer(buffer, "{\n", filepath.Dir(k.FuryctlConfPath)); err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}

		if np.Type != nil {
			if err := bytesx.SafeWriteToBuffer(
				buffer,
				"type = \"%v\"\n",
				filepath.Dir(k.FuryctlConfPath),
				*np.Type,
			); err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}
		}

		if err := bytesx.SafeWriteToBuffer(
			buffer,
			"name = \"%v\"\n",
			filepath.Dir(k.FuryctlConfPath),
			np.Name,
		); err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}

		if err := bytesx.SafeWriteToBuffer(
			buffer,
			"version = null\n",
			filepath.Dir(k.FuryctlConfPath),
		); err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}

		if np.Ami != nil {
			if err := bytesx.SafeWriteToBuffer(
				buffer,
				"ami_id = \"%v\"\n",
				filepath.Dir(k.FuryctlConfPath),
				np.Ami.Id,
			); err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}
		}

		spot := "false"

		if np.Instance.Spot != nil {
			spot = strconv.FormatBool(*np.Instance.Spot)
		}

		if err := bytesx.SafeWriteToBuffer(
			buffer,
			"spot_instance = %v\n",
			filepath.Dir(k.FuryctlConfPath),
			spot,
		); err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}

		if np.ContainerRuntime != nil {
			if err := bytesx.SafeWriteToBuffer(
				buffer,
				"container_runtime = \"%v\"\n",
				filepath.Dir(k.FuryctlConfPath),
				*np.ContainerRuntime,
			); err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}
		}

		if err := bytesx.SafeWriteToBuffer(
			buffer,
			"min_size = %v\n",
			filepath.Dir(k.FuryctlConfPath),
			np.Size.Min,
		); err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}

		if err := bytesx.SafeWriteToBuffer(
			buffer,
			"max_size = %v\n",
			filepath.Dir(k.FuryctlConfPath),
			np.Size.Max,
		); err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}

		if err := bytesx.SafeWriteToBuffer(
			buffer,
			"instance_type = \"%v\"\n",
			filepath.Dir(k.FuryctlConfPath),
			np.Instance.Type,
		); err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}

		if err := k.addAttachedTargetGroupsToNodePool(buffer, np.AttachedTargetGroups); err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}

		if np.Instance.MaxPods != nil {
			if err := bytesx.SafeWriteToBuffer(
				buffer,
				"max_pods = %v\n",
				filepath.Dir(k.FuryctlConfPath),
				*np.Instance.MaxPods,
			); err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}
		}

		if err := k.addVolumeSizeToNodePool(buffer, np.Instance.VolumeSize); err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}

		if err := k.addSubnetIdsToNodePool(buffer, np.SubnetIds); err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}

		if err := k.addFirewallRulesToNodePool(buffer, np); err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}

		if err := k.addLabelsToNodePool(buffer, np.Labels); err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}

		if err := k.addTaintsToNodePool(buffer, np.Taints); err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}

		if err := k.addTagsToNodePool(buffer, np.Tags); err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}

		if err := bytesx.SafeWriteToBuffer(
			buffer,
			"},\n",
			filepath.Dir(k.FuryctlConfPath),
		); err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	}

	if err := bytesx.SafeWriteToBuffer(
		buffer,
		"]\n",
		filepath.Dir(k.FuryctlConfPath),
	); err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	return nil
}

func (k *Kubernetes) addVolumeSizeToNodePool(buffer *bytes.Buffer, vs *int) error {
	volumeSize := nodePoolDefaultVolumeSize

	if vs != nil {
		volumeSize = *vs
	}

	if err := bytesx.SafeWriteToBuffer(
		buffer,
		"volume_size = %v\n",
		filepath.Dir(k.FuryctlConfPath),
		volumeSize,
	); err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	return nil
}

func (k *Kubernetes) addAttachedTargetGroupsToNodePool(buffer *bytes.Buffer, atgs []private.TypesAwsArn) error {
	if len(atgs) > 0 {
		attachedTargetGroups := make([]string, len(atgs))

		for i, tg := range atgs {
			attachedTargetGroups[i] = fmt.Sprintf("\"%v\"", tg)
		}

		if err := bytesx.SafeWriteToBuffer(
			buffer,
			"target_group_arns = [%v]\n",
			filepath.Dir(k.FuryctlConfPath),
			strings.Join(attachedTargetGroups, ","),
		); err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	}

	return nil
}

func (k *Kubernetes) addSubnetIdsToNodePool(buffer *bytes.Buffer, subnetIds []private.TypesAwsSubnetId) error {
	if len(subnetIds) > 0 {
		npSubNetIds := make([]string, len(subnetIds))

		for i, subnetID := range subnetIds {
			npSubNetIds[i] = fmt.Sprintf("\"%v\"", subnetID)
		}

		if err := bytesx.SafeWriteToBuffer(
			buffer,
			"subnets = [%v]\n",
			filepath.Dir(k.FuryctlConfPath),
			strings.Join(npSubNetIds, ","),
		); err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	} else {
		if err := bytesx.SafeWriteToBuffer(
			buffer,
			"subnets = null\n",
			filepath.Dir(k.FuryctlConfPath),
		); err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	}

	return nil
}

func (k *Kubernetes) addLabelsToNodePool(buffer *bytes.Buffer, labels private.TypesKubeLabels) error {
	if len(labels) > 0 {
		l, err := json.Marshal(labels)
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}

		if err := bytesx.SafeWriteToBuffer(
			buffer,
			"labels = %v\n",
			filepath.Dir(k.FuryctlConfPath),
			string(l),
		); err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	} else {
		if err := bytesx.SafeWriteToBuffer(
			buffer,
			"labels = null\n",
			filepath.Dir(k.FuryctlConfPath),
		); err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	}

	return nil
}

func (k *Kubernetes) addTaintsToNodePool(buffer *bytes.Buffer, taints private.TypesKubeTaints) error {
	if len(taints) > 0 {
		if err := bytesx.SafeWriteToBuffer(
			buffer,
			"taints = [\"%v\"]\n",
			filepath.Dir(k.FuryctlConfPath),
			strings.Join(taints, "\",\""),
		); err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	} else {
		if err := bytesx.SafeWriteToBuffer(
			buffer,
			"taints = null\n",
			filepath.Dir(k.FuryctlConfPath),
		); err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	}

	return nil
}

func (k *Kubernetes) addTagsToNodePool(buffer *bytes.Buffer, tags private.TypesAwsTags) error {
	if len(tags) > 0 {
		t, err := json.Marshal(tags)
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}

		if err := bytesx.SafeWriteToBuffer(
			buffer,
			"tags = %v\n",
			filepath.Dir(k.FuryctlConfPath),
			string(t),
		); err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	} else {
		if err := bytesx.SafeWriteToBuffer(
			buffer,
			"tags = null\n",
			filepath.Dir(k.FuryctlConfPath),
		); err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	}

	return nil
}

func (k *Kubernetes) addFirewallRulesToNodePool(buffer *bytes.Buffer, np private.SpecKubernetesNodePool) error {
	if np.AdditionalFirewallRules != nil {
		err := bytesx.SafeWriteToBuffer(
			buffer,
			"additional_firewall_rules = {\n",
			filepath.Dir(k.FuryctlConfPath),
		)
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}

		if len(np.AdditionalFirewallRules.CidrBlocks) > 0 {
			if err := bytesx.SafeWriteToBuffer(
				buffer,
				"cidr_blocks = [\n",
				filepath.Dir(k.FuryctlConfPath),
			); err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}

			if err = k.addCidrBlocksFirewallRules(buffer, np.AdditionalFirewallRules.CidrBlocks); err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}

			if err = bytesx.SafeWriteToBuffer(
				buffer,
				"]\n",
				filepath.Dir(k.FuryctlConfPath),
			); err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}
		}

		if len(np.AdditionalFirewallRules.SourceSecurityGroupId) > 0 {
			if err := bytesx.SafeWriteToBuffer(
				buffer,
				"source_security_group_id = [\n",
				filepath.Dir(k.FuryctlConfPath),
			); err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}

			if err = k.addSourceSecurityGroupIDFirewallRules(
				buffer, np.AdditionalFirewallRules.SourceSecurityGroupId,
			); err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}

			if err = bytesx.SafeWriteToBuffer(
				buffer,
				"]\n",
				filepath.Dir(k.FuryctlConfPath),
			); err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}
		}

		if len(np.AdditionalFirewallRules.Self) > 0 {
			if err := bytesx.SafeWriteToBuffer(
				buffer,
				"self = [\n",
				filepath.Dir(k.FuryctlConfPath),
			); err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}

			if err = k.addSelfFirewallRules(buffer, np.AdditionalFirewallRules.Self); err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}

			if err = bytesx.SafeWriteToBuffer(
				buffer,
				"]\n",
				filepath.Dir(k.FuryctlConfPath),
			); err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}
		}

		err = bytesx.SafeWriteToBuffer(
			buffer,
			"}\n",
			filepath.Dir(k.FuryctlConfPath),
		)
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	} else {
		err := bytesx.SafeWriteToBuffer(
			buffer,
			"additional_firewall_rules = null\n",
			filepath.Dir(k.FuryctlConfPath),
		)
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	}

	return nil
}

func (k *Kubernetes) addCidrBlocksFirewallRules(
	buffer *bytes.Buffer,
	cb []private.SpecKubernetesNodePoolAdditionalFirewallRuleCidrBlock,
) error {
	for i, fwRule := range cb {
		fwRuleTags := "{}"

		if len(fwRule.Tags) > 0 {
			var tags []byte

			tags, err := json.Marshal(fwRule.Tags)
			if err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}

			fwRuleTags = string(tags)
		}

		content := `{
	description = "%v"
	type = "%v"
	cidr_blocks = %v
	protocol = "%v"
	from_port = "%v"
	to_port = "%v"
	tags = %v
}`

		if i < len(cb)-1 {
			content += ","
		}

		uniqCidrBlocks := slices.Uniq(fwRule.CidrBlocks)

		dmzCidrRanges := make([]string, len(uniqCidrBlocks))

		for i, cidr := range uniqCidrBlocks {
			dmzCidrRanges[i] = fmt.Sprintf("\"%v\"", cidr)
		}

		if err := bytesx.SafeWriteToBuffer(
			buffer,
			content,
			filepath.Dir(k.FuryctlConfPath),
			fwRule.Name,
			fwRule.Type,
			fmt.Sprintf("[%v]", strings.Join(dmzCidrRanges, ",")),
			fwRule.Protocol,
			fwRule.Ports.From,
			fwRule.Ports.To,
			fwRuleTags,
		); err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	}

	return nil
}

func (k *Kubernetes) addSourceSecurityGroupIDFirewallRules(
	buffer *bytes.Buffer,
	cb []private.SpecKubernetesNodePoolAdditionalFirewallRuleSourceSecurityGroupId,
) error {
	for i, fwRule := range cb {
		fwRuleTags := "{}"

		if len(fwRule.Tags) > 0 {
			var tags []byte

			tags, err := json.Marshal(fwRule.Tags)
			if err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}

			fwRuleTags = string(tags)
		}

		content := `{
	description = "%v"
	type = "%v"
	source_security_group_id = %v
	protocol = "%v"
	from_port = "%v"
	to_port = "%v"
	tags = %v
}`

		if i < len(cb)-1 {
			content += ","
		}

		if err := bytesx.SafeWriteToBuffer(
			buffer,
			content,
			filepath.Dir(k.FuryctlConfPath),
			fwRule.Name,
			fwRule.Type,
			fwRule.SourceSecurityGroupId,
			fwRule.Protocol,
			fwRule.Ports.From,
			fwRule.Ports.To,
			fwRuleTags,
		); err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	}

	return nil
}

func (k *Kubernetes) addSelfFirewallRules(
	buffer *bytes.Buffer,
	cb []private.SpecKubernetesNodePoolAdditionalFirewallRuleSelf,
) error {
	for i, fwRule := range cb {
		fwRuleTags := "{}"

		if len(fwRule.Tags) > 0 {
			var tags []byte

			tags, err := json.Marshal(fwRule.Tags)
			if err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}

			fwRuleTags = string(tags)
		}

		content := `{
	description = "%v"
	type = "%v"
	self = %t
	protocol = "%v"
	from_port = "%v"
	to_port = "%v"
	tags = %v
}`

		if i < len(cb)-1 {
			content += ","
		}

		if err := bytesx.SafeWriteToBuffer(
			buffer,
			content,
			filepath.Dir(k.FuryctlConfPath),
			fwRule.Name,
			fwRule.Type,
			fwRule.Self,
			fwRule.Protocol,
			fwRule.Ports.From,
			fwRule.Ports.To,
			fwRuleTags,
		); err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	}

	return nil
}
