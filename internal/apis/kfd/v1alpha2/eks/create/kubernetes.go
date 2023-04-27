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
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/config"
	"github.com/sighupio/fury-distribution/pkg/schema/private"
	"github.com/sighupio/furyctl/configs"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/merge"
	"github.com/sighupio/furyctl/internal/template"
	"github.com/sighupio/furyctl/internal/tool/awscli"
	"github.com/sighupio/furyctl/internal/tool/terraform"
	bytesx "github.com/sighupio/furyctl/internal/x/bytes"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
	kubex "github.com/sighupio/furyctl/internal/x/kube"
	netx "github.com/sighupio/furyctl/internal/x/net"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

var (
	errKubeconfigFromLogs = errors.New("cannot get kubeconfig file after cluster creation")
	errPvtSubnetNotFound  = errors.New("private_subnets not found in infrastructure phase's output")
	errPvtSubnetFromOut   = errors.New("cannot read private_subnets from infrastructure's output.json")
	errVpcCIDRFromOut     = errors.New("cannot read vpc_cidr_block from infrastructure's output.json")
	errVpcCIDRNotFound    = errors.New("vpc_cidr_block not found in infra output")
	errVpcIDNotFound      = errors.New("vpcId not found: you forgot to specify one in the configuration file " +
		"or the infrastructure phase failed")
	errParsingCIDR         = errors.New("error parsing CIDR")
	errResolvingDNS        = errors.New("error resolving DNS record")
	errVpcIDNotProvided    = errors.New("vpcId not provided")
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
	furyctlConf      private.EksclusterKfdV1Alpha2
	kfdManifest      config.KFD
	infraOutputsPath string
	distroPath       string
	furyctlConfPath  string
	tfRunner         *terraform.Runner
	awsRunner        *awscli.Runner
	dryRun           bool
}

func NewKubernetes(
	furyctlConf private.EksclusterKfdV1Alpha2,
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
		distroPath:       paths.DistroPath,
		furyctlConfPath:  paths.ConfigPath,
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

	cfg, err := k.mergeConfig()
	if err != nil {
		return fmt.Errorf("error merging furyctl configuration: %w", err)
	}

	if err := k.copyFromTemplate(cfg); err != nil {
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

	if k.furyctlConf.Spec.Kubernetes.ApiServer.PrivateAccess &&
		!k.furyctlConf.Spec.Kubernetes.ApiServer.PublicAccess {
		logrus.Info("Checking connection to the VPC...")

		if err := k.checkVPCConnection(); err != nil {
			logrus.Debugf("error checking VPC connection: %v", err)

			if k.furyctlConf.Spec.Infrastructure != nil {
				if k.furyctlConf.Spec.Infrastructure.Vpn != nil {
					return fmt.Errorf("%w please check your VPN connection and try again", errKubeAPIUnreachable)
				}
			}

			return fmt.Errorf("%w please check your VPC configuration and try again", errKubeAPIUnreachable)
		}
	}

	logrus.Warn("Creating cloud resources, this could take a while...")

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

func (k *Kubernetes) Stop() error {
	errCh := make(chan error)
	doneCh := make(chan bool)

	var wg sync.WaitGroup

	//nolint:gomnd // ignore magic number linters
	wg.Add(2)

	go func() {
		logrus.Debug("Stopping terraform...")

		if err := k.tfRunner.Stop(); err != nil {
			errCh <- fmt.Errorf("error stopping terraform: %w", err)
		}

		wg.Done()
	}()

	go func() {
		logrus.Debug("Stopping awscli...")

		if err := k.awsRunner.Stop(); err != nil {
			errCh <- fmt.Errorf("error stopping awscli: %w", err)
		}

		wg.Done()
	}()

	go func() {
		wg.Wait()
		close(doneCh)
	}()

	select {
	case <-doneCh:

	case err := <-errCh:
		close(errCh)

		return err
	}

	return nil
}

func (k *Kubernetes) copyFromTemplate(furyctlCfg template.Config) error {
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

	eksInstallerPath := path.Join(k.Path, "..", "vendor", "installers", "eks", "modules", "eks")

	nodeSelector, tolerations, err := k.getCommonDataFromDistribution(furyctlCfg)
	if err != nil {
		return err
	}

	tfConfVars := map[string]map[any]any{
		"spec": {
			"region": k.furyctlConf.Spec.Region,
			"tags":   k.furyctlConf.Spec.Tags,
		},
		"kubernetes": {
			"installerPath": eksInstallerPath,
			"tfVersion":     k.kfdManifest.Tools.Common.Terraform.Version,
		},
		"distribution": {
			"nodeSelector": nodeSelector,
			"tolerations":  tolerations,
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

func (k *Kubernetes) mergeConfig() (template.Config, error) {
	var cfg template.Config

	defaultsFilePath := path.Join(k.distroPath, "furyctl-defaults.yaml")

	defaultsFile, err := yamlx.FromFileV2[map[any]any](defaultsFilePath)
	if err != nil {
		return cfg, fmt.Errorf("%s - %w", defaultsFilePath, err)
	}

	furyctlConf, err := yamlx.FromFileV2[map[any]any](k.furyctlConfPath)
	if err != nil {
		return cfg, fmt.Errorf("%s - %w", k.furyctlConfPath, err)
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

//nolint:gocyclo,maintidx,funlen // it will be refactored
func (k *Kubernetes) createTfVars() error {
	var buffer bytes.Buffer

	subnetIdsSource := k.furyctlConf.Spec.Kubernetes.SubnetIds
	vpcIDSource := k.furyctlConf.Spec.Kubernetes.VpcId

	allowedClusterEndpointPrivateAccessCIDRs := k.furyctlConf.Spec.Kubernetes.ApiServer.PrivateAccessCidrs
	allowedClusterEndpointPublicAccessCIDRs := k.furyctlConf.Spec.Kubernetes.ApiServer.PublicAccessCidrs

	if k.furyctlConf.Spec.Infrastructure != nil &&
		k.furyctlConf.Spec.Infrastructure.Vpc != nil {
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

				subs := make([]private.TypesAwsSubnetId, len(s))

				for i, sub := range s {
					ss, ok := sub.(string)
					if !ok {
						return errPvtSubnetFromOut
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
		"kubectl_path = \"%s\"\n",
		k.KubectlPath,
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
		"cluster_endpoint_private_access = %v\n",
		k.furyctlConf.Spec.Kubernetes.ApiServer.PrivateAccess,
	)
	if err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	clusterEndpointPrivateAccessCidrs := make([]string, len(allowedClusterEndpointPrivateAccessCIDRs))

	for i, cidr := range allowedClusterEndpointPrivateAccessCIDRs {
		clusterEndpointPrivateAccessCidrs[i] = fmt.Sprintf("\"%v\"", cidr)
	}

	err = bytesx.SafeWriteToBuffer(
		&buffer,
		"cluster_endpoint_private_access_cidrs = [%v]\n",
		strings.Join(clusterEndpointPrivateAccessCidrs, ","),
	)
	if err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	err = bytesx.SafeWriteToBuffer(
		&buffer,
		"cluster_endpoint_public_access = %v\n",
		k.furyctlConf.Spec.Kubernetes.ApiServer.PublicAccess,
	)
	if err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	clusterEndpointPublicAccessCidrs := make([]string, len(allowedClusterEndpointPublicAccessCIDRs))

	for i, cidr := range allowedClusterEndpointPublicAccessCIDRs {
		clusterEndpointPublicAccessCidrs[i] = fmt.Sprintf("\"%v\"", cidr)
	}

	err = bytesx.SafeWriteToBuffer(
		&buffer,
		"cluster_endpoint_public_access_cidrs = [%v]\n",
		strings.Join(clusterEndpointPublicAccessCidrs, ","),
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

		vpcIDSource = new(private.TypesAwsVpcId)
	}

	err = bytesx.SafeWriteToBuffer(
		&buffer,
		"vpc_id = \"%v\"\n",
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
		"subnets = [%v]\n",
		strings.Join(subnetIds, ","),
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

			if np.Ami != nil {
				err = bytesx.SafeWriteToBuffer(
					&buffer,
					"ami_id = \"%v\"\n",
					np.Ami.Id,
				)
				if err != nil {
					return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
				}
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

			if np.ContainerRuntime != nil {
				err = bytesx.SafeWriteToBuffer(
					&buffer,
					"container_runtime = \"%v\"\n",
					*np.ContainerRuntime,
				)
				if err != nil {
					return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
				}
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
					"target_group_arns = [%v]\n",
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
					"subnets = [%v]\n",
					strings.Join(npSubNetIds, ","),
				)
				if err != nil {
					return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
				}
			} else {
				err = bytesx.SafeWriteToBuffer(
					&buffer,
					"subnets = null\n",
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
					"labels = null\n",
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
					"taints = null\n",
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
					"tags = null\n",
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

func (k *Kubernetes) addFirewallRulesToNodePool(buffer *bytes.Buffer, np private.SpecKubernetesNodePool) error {
	if np.AdditionalFirewallRules != nil {
		err := bytesx.SafeWriteToBuffer(
			buffer,
			"additional_firewall_rules = {\n",
		)
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}

		if len(np.AdditionalFirewallRules.CidrBlocks) > 0 {
			if err := bytesx.SafeWriteToBuffer(
				buffer,
				"cidr_blocks = [\n",
			); err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}

			if err = k.addCidrBlocksFirewallRules(buffer, np.AdditionalFirewallRules.CidrBlocks); err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}

			if err = bytesx.SafeWriteToBuffer(
				buffer,
				"]\n",
			); err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}
		}

		if len(np.AdditionalFirewallRules.SourceSecurityGroupId) > 0 {
			if err := bytesx.SafeWriteToBuffer(
				buffer,
				"source_security_group_id = [\n",
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
			); err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}
		}

		if len(np.AdditionalFirewallRules.Self) > 0 {
			if err := bytesx.SafeWriteToBuffer(
				buffer,
				"self = [\n",
			); err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}

			if err = k.addSelfFirewallRules(buffer, np.AdditionalFirewallRules.Self); err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}

			if err = bytesx.SafeWriteToBuffer(
				buffer,
				"]\n",
			); err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}
		}

		err = bytesx.SafeWriteToBuffer(
			buffer,
			"}\n",
		)
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	} else {
		err := bytesx.SafeWriteToBuffer(
			buffer,
			"additional_firewall_rules = null\n",
		)
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	}

	return nil
}

func (*Kubernetes) addCidrBlocksFirewallRules(
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

		content := "{\ndescription = \"%v\"\ntype = \"%v\"\ncidr_blocks = %v\nprotocol = \"%v\"\n" +
			"from_port = \"%v\"\nto_port = \"%v\"\ntags = %v\n}"

		if i < len(cb)-1 {
			content += ","
		}

		dmzCidrRanges := make([]string, len(fwRule.CidrBlocks))

		for i, cidr := range fwRule.CidrBlocks {
			dmzCidrRanges[i] = fmt.Sprintf("\"%v\"", cidr)
		}

		if err := bytesx.SafeWriteToBuffer(
			buffer,
			content,
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

func (*Kubernetes) addSourceSecurityGroupIDFirewallRules(
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

		content := "{\ndescription = \"%v\"\ntype = \"%v\"\nsource_security_group_id = %v\nprotocol = \"%v\"\n" +
			"from_port = \"%v\"\nto_port = \"%v\"\ntags = %v\n}"

		if i < len(cb)-1 {
			content += ","
		}

		if err := bytesx.SafeWriteToBuffer(
			buffer,
			content,
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

func (*Kubernetes) addSelfFirewallRules(
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

		content := "{\ndescription = \"%v\"\ntype = \"%v\"\nself = %t\nprotocol = \"%v\"\n" +
			"from_port = \"%v\"\nto_port = \"%v\"\ntags = %v\n}"

		if i < len(cb)-1 {
			content += ","
		}

		if err := bytesx.SafeWriteToBuffer(
			buffer,
			content,
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
			"--region",
			string(k.furyctlConf.Spec.Region),
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
