// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package create

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/fury-distribution/pkg/apis/ekscluster/v1alpha2/private"
	"github.com/sighupio/furyctl/configs"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/eks"
	"github.com/sighupio/furyctl/internal/merge"
	"github.com/sighupio/furyctl/internal/parser"
	"github.com/sighupio/furyctl/internal/template"
	"github.com/sighupio/furyctl/internal/tool/awscli"
	"github.com/sighupio/furyctl/internal/tool/terraform"
	"github.com/sighupio/furyctl/internal/upgrade"
	bytesx "github.com/sighupio/furyctl/internal/x/bytes"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
	kubex "github.com/sighupio/furyctl/internal/x/kube"
	netx "github.com/sighupio/furyctl/internal/x/net"
	"github.com/sighupio/furyctl/internal/x/slices"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

var (
	errMissingKubeconfig = errors.New("kubeconfig not found in infrastructure phase's logs")
	errWrongKubeconfig   = errors.New("kubeconfig cannot be parsed from infrastructure phase's logs")
	errPvtSubnetNotFound = errors.New("private_subnets not found in infrastructure phase's output")
	errPvtSubnetFromOut  = errors.New("cannot read private_subnets from infrastructure's output.json")
	errVpcCIDRFromOut    = errors.New("cannot read vpc_cidr_block from infrastructure's output.json")
	errVpcCIDRNotFound   = errors.New("vpc_cidr_block not found in infra output")
	errVpcIDNotFound     = errors.New("vpcId not found: you forgot to specify one in the configuration file " +
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
	upgrade          *upgrade.Upgrade
}

func NewKubernetes(
	furyctlConf private.EksclusterKfdV1Alpha2,
	kfdManifest config.KFD,
	infraOutputsPath string,
	paths cluster.CreatorPaths,
	dryRun bool,
	upgr *upgrade.Upgrade,
) *Kubernetes {
	kubeDir := path.Join(paths.WorkDir, cluster.OperationPhaseKubernetes)

	phase := cluster.NewOperationPhase(kubeDir, kfdManifest.Tools, paths.BinPath)

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
				Logs:      phase.TerraformLogsPath,
				Outputs:   phase.TerraformOutputsPath,
				WorkDir:   path.Join(phase.Path, "terraform"),
				Plan:      phase.TerraformPlanPath,
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
		dryRun:  dryRun,
		upgrade: upgr,
	}
}

func (k *Kubernetes) Exec(startFrom string, upgradeState *upgrade.State) error {
	timestamp := time.Now().Unix()

	logrus.Info("Creating Kubernetes Fury cluster...")

	logrus.Debug("Create: running kubernetes phase...")

	if err := k.prepare(); err != nil {
		return fmt.Errorf("error preparing kubernetes phase: %w", err)
	}

	if err := k.preKubernetes(startFrom, upgradeState); err != nil {
		return fmt.Errorf("error running pre-kubernetes phase: %w", err)
	}

	if err := k.coreKubernetes(startFrom, upgradeState, timestamp); err != nil {
		return fmt.Errorf("error running core kubernetes phase: %w", err)
	}

	if k.dryRun {
		return nil
	}

	if err := k.postKubernetes(upgradeState); err != nil {
		return fmt.Errorf("error running post-kubernetes phase: %w", err)
	}

	return nil
}

func (k *Kubernetes) prepare() error {
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

	return nil
}

func (k *Kubernetes) preKubernetes(
	startFrom string,
	upgradeState *upgrade.State,
) error {
	if !k.dryRun && (startFrom == "" || startFrom == cluster.OperationSubPhasePreKubernetes) {
		if err := k.upgrade.Exec(k.Path, "pre-kubernetes"); err != nil {
			upgradeState.Phases.PreKubernetes.Status = upgrade.PhaseStatusFailed

			return fmt.Errorf("error running upgrade: %w", err)
		}

		if k.upgrade.Enabled {
			upgradeState.Phases.PreKubernetes.Status = upgrade.PhaseStatusSuccess
		}
	}

	return nil
}

func (k *Kubernetes) coreKubernetes(
	startFrom string,
	upgradeState *upgrade.State,
	timestamp int64,
) error {
	if startFrom != cluster.OperationSubPhasePostKubernetes {
		plan, err := k.tfRunner.Plan(timestamp)
		if err != nil {
			return fmt.Errorf("error running terraform plan: %w", err)
		}

		if k.dryRun {
			return nil
		}

		tfParser := parser.NewTfPlanParser(string(plan))

		parsedPlan := tfParser.Parse()

		eksKube := eks.NewKubernetes()

		criticalResources := slices.Intersection(eksKube.GetCriticalTFResourceTypes(), parsedPlan.Destroy)

		if len(criticalResources) > 0 {
			logrus.Warnf("Deletion of the following critical resources has been detected: %s. See the logs for more details.",
				strings.Join(criticalResources, ", "))
			logrus.Warn("Do you want to proceed? write 'yes' to continue or anything else to abort: ")

			prompter := iox.NewPrompter(bufio.NewReader(os.Stdin))

			prompt, err := prompter.Ask("yes")
			if err != nil {
				return fmt.Errorf("error reading user input: %w", err)
			}

			if !prompt {
				return ErrAbortedByUser
			}
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

		if err := k.tfRunner.Apply(timestamp); err != nil {
			if k.upgrade.Enabled {
				upgradeState.Phases.Kubernetes.Status = upgrade.PhaseStatusFailed
			}

			return fmt.Errorf("cannot create cloud resources: %w", err)
		}

		if k.upgrade.Enabled {
			upgradeState.Phases.Kubernetes.Status = upgrade.PhaseStatusSuccess
		}

		out, err := k.tfRunner.Output()
		if err != nil {
			return fmt.Errorf("error getting terraform output: %w", err)
		}

		if out["kubeconfig"] == nil {
			return errMissingKubeconfig
		}

		kubeString, ok := out["kubeconfig"].Value.(string)
		if !ok {
			return errWrongKubeconfig
		}

		p, err := kubex.CreateConfig([]byte(kubeString), k.TerraformSecretsPath)
		if err != nil {
			return fmt.Errorf("error creating kubeconfig: %w", err)
		}

		if err := kubex.SetConfigEnv(p); err != nil {
			return fmt.Errorf("error setting kubeconfig env: %w", err)
		}

		if err := kubex.CopyToWorkDir(p, "kubeconfig"); err != nil {
			return fmt.Errorf("error copying kubeconfig: %w", err)
		}
	}

	return nil
}

func (k *Kubernetes) postKubernetes(
	upgradeState *upgrade.State,
) error {
	if err := k.upgrade.Exec(k.Path, "post-kubernetes"); err != nil {
		upgradeState.Phases.PostKubernetes.Status = upgrade.PhaseStatusFailed

		return fmt.Errorf("error running upgrade: %w", err)
	}

	if k.upgrade.Enabled {
		upgradeState.Phases.PostKubernetes.Status = upgrade.PhaseStatusSuccess
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
					"bucketName":           k.furyctlConf.Spec.ToolsConfiguration.Terraform.State.S3.BucketName,
					"keyPrefix":            k.furyctlConf.Spec.ToolsConfiguration.Terraform.State.S3.KeyPrefix,
					"region":               k.furyctlConf.Spec.ToolsConfiguration.Terraform.State.S3.Region,
					"skipRegionValidation": k.furyctlConf.Spec.ToolsConfiguration.Terraform.State.S3.SkipRegionValidation,
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
		k.furyctlConfPath,
	)
	if err != nil {
		return fmt.Errorf("error generating from template files: %w", err)
	}

	return nil
}

func (k *Kubernetes) mergeConfig() (template.Config, error) {
	var cfg template.Config

	defaultsFilePath := path.Join(k.distroPath, "defaults", "ekscluster-kfd-v1alpha2.yaml")

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

//nolint:gocyclo,maintidx // it will be refactored
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
				if infraOut["private_subnets"] == nil {
					return errPvtSubnetNotFound
				}

				s, ok := infraOut["private_subnets"].Value.([]any)
				if !ok {
					return errPvtSubnetFromOut
				}

				if infraOut["vpc_id"] == nil {
					return ErrVpcIDNotFound
				}

				v, ok := infraOut["vpc_id"].Value.(string)
				if !ok {
					return ErrVpcIDFromOut
				}

				if infraOut["vpc_cidr_block"] == nil {
					return errVpcCIDRNotFound
				}

				c, ok := infraOut["vpc_cidr_block"].Value.(string)
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

	allowedClusterEndpointPrivateAccessCIDRs = slices.Uniq(allowedClusterEndpointPrivateAccessCIDRs)

	if err := bytesx.SafeWriteToBuffer(
		&buffer,
		"cluster_name = \"%v\"\n",
		filepath.Dir(k.furyctlConfPath),
		k.furyctlConf.Metadata.Name,
	); err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	if err := bytesx.SafeWriteToBuffer(
		&buffer,
		"kubectl_path = \"%s\"\n",
		filepath.Dir(k.furyctlConfPath),
		k.KubectlPath,
	); err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	if err := bytesx.SafeWriteToBuffer(
		&buffer,
		"cluster_version = \"%v\"\n",
		filepath.Dir(k.furyctlConfPath),
		k.kfdManifest.Kubernetes.Eks.Version,
	); err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	if err := bytesx.SafeWriteToBuffer(
		&buffer,
		"cluster_endpoint_private_access = %v\n",
		filepath.Dir(k.furyctlConfPath),
		k.furyctlConf.Spec.Kubernetes.ApiServer.PrivateAccess,
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
		filepath.Dir(k.furyctlConfPath),
		strings.Join(clusterEndpointPrivateAccessCidrs, ","),
	); err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	if err := bytesx.SafeWriteToBuffer(
		&buffer,
		"cluster_endpoint_public_access = %v\n",
		filepath.Dir(k.furyctlConfPath),
		k.furyctlConf.Spec.Kubernetes.ApiServer.PublicAccess,
	); err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	if k.furyctlConf.Spec.Kubernetes.ApiServer.PublicAccess && len(allowedClusterEndpointPublicAccessCIDRs) == 0 {
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
		filepath.Dir(k.furyctlConfPath),
		strings.Join(clusterEndpointPublicAccessCidrs, ","),
	); err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	if k.furyctlConf.Spec.Kubernetes.ServiceIpV4Cidr == nil {
		if err := bytesx.SafeWriteToBuffer(
			&buffer,
			"cluster_service_ipv4_cidr = null\n",
			filepath.Dir(k.furyctlConfPath),
		); err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	} else {
		if err := bytesx.SafeWriteToBuffer(
			&buffer,
			"cluster_service_ipv4_cidr = \"%v\"\n",
			filepath.Dir(k.furyctlConfPath),
			k.furyctlConf.Spec.Kubernetes.ServiceIpV4Cidr,
		); err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	}

	if err := bytesx.SafeWriteToBuffer(
		&buffer,
		"node_pools_launch_kind = \"%v\"\n",
		filepath.Dir(k.furyctlConfPath),
		k.furyctlConf.Spec.Kubernetes.NodePoolsLaunchKind,
	); err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	if k.furyctlConf.Spec.Kubernetes.LogRetentionDays != nil {
		if err := bytesx.SafeWriteToBuffer(
			&buffer,
			"cluster_log_retention_days = %v\n",
			filepath.Dir(k.furyctlConfPath),
			*k.furyctlConf.Spec.Kubernetes.LogRetentionDays,
		); err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	}

	if vpcIDSource == nil {
		if !k.dryRun {
			return errVpcIDNotFound
		}

		vpcIDSource = new(private.TypesAwsVpcId)
	}

	if err := bytesx.SafeWriteToBuffer(
		&buffer,
		"vpc_id = \"%v\"\n",
		filepath.Dir(k.furyctlConfPath),
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
		filepath.Dir(k.furyctlConfPath),
		strings.Join(subnetIds, ","),
	); err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	if err := bytesx.SafeWriteToBuffer(
		&buffer,
		"ssh_public_key = \"%v\"\n",
		filepath.Dir(k.furyctlConfPath),
		k.furyctlConf.Spec.Kubernetes.NodeAllowedSshPublicKey,
	); err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	if err := k.addAwsAuthToTfVars(&buffer); err != nil {
		return fmt.Errorf("error writing AWS Auth to Terraform vars file: %w", err)
	}

	if len(k.furyctlConf.Spec.Kubernetes.NodePools) > 0 {
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

func (k *Kubernetes) addNodePoolsToTfVars(buffer *bytes.Buffer) error {
	if err := bytesx.SafeWriteToBuffer(buffer, "node_pools = [\n", filepath.Dir(k.furyctlConfPath)); err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	for _, np := range k.furyctlConf.Spec.Kubernetes.NodePools {
		if err := bytesx.SafeWriteToBuffer(buffer, "{\n", filepath.Dir(k.furyctlConfPath)); err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}

		if np.Type != nil {
			if err := bytesx.SafeWriteToBuffer(
				buffer,
				"type = \"%v\"\n",
				filepath.Dir(k.furyctlConfPath),
				*np.Type,
			); err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}
		}

		if err := bytesx.SafeWriteToBuffer(
			buffer,
			"name = \"%v\"\n",
			filepath.Dir(k.furyctlConfPath),
			np.Name,
		); err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}

		if err := bytesx.SafeWriteToBuffer(
			buffer,
			"version = null\n",
			filepath.Dir(k.furyctlConfPath),
		); err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}

		if np.Ami != nil {
			if err := bytesx.SafeWriteToBuffer(
				buffer,
				"ami_id = \"%v\"\n",
				filepath.Dir(k.furyctlConfPath),
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
			filepath.Dir(k.furyctlConfPath),
			spot,
		); err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}

		if np.ContainerRuntime != nil {
			if err := bytesx.SafeWriteToBuffer(
				buffer,
				"container_runtime = \"%v\"\n",
				filepath.Dir(k.furyctlConfPath),
				*np.ContainerRuntime,
			); err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}
		}

		if err := bytesx.SafeWriteToBuffer(
			buffer,
			"min_size = %v\n",
			filepath.Dir(k.furyctlConfPath),
			np.Size.Min,
		); err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}

		if err := bytesx.SafeWriteToBuffer(
			buffer,
			"max_size = %v\n",
			filepath.Dir(k.furyctlConfPath),
			np.Size.Max,
		); err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}

		if err := bytesx.SafeWriteToBuffer(
			buffer,
			"instance_type = \"%v\"\n",
			filepath.Dir(k.furyctlConfPath),
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
				filepath.Dir(k.furyctlConfPath),
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
			filepath.Dir(k.furyctlConfPath),
		); err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	}

	if err := bytesx.SafeWriteToBuffer(
		buffer,
		"]\n",
		filepath.Dir(k.furyctlConfPath),
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
		filepath.Dir(k.furyctlConfPath),
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
			filepath.Dir(k.furyctlConfPath),
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
			filepath.Dir(k.furyctlConfPath),
			strings.Join(npSubNetIds, ","),
		); err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	} else {
		if err := bytesx.SafeWriteToBuffer(
			buffer,
			"subnets = null\n",
			filepath.Dir(k.furyctlConfPath),
		); err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	}

	return nil
}

func (k *Kubernetes) addLabelsToNodePool(buffer *bytes.Buffer, labels private.TypesKubeLabels) error {
	if len(labels) > 0 {
		var uLabels []byte

		l, err := json.Marshal(uLabels)
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}

		if err := bytesx.SafeWriteToBuffer(
			buffer,
			"labels = %v\n",
			filepath.Dir(k.furyctlConfPath),
			string(l),
		); err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	} else {
		if err := bytesx.SafeWriteToBuffer(
			buffer,
			"labels = null\n",
			filepath.Dir(k.furyctlConfPath),
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
			filepath.Dir(k.furyctlConfPath),
			strings.Join(taints, "\",\""),
		); err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	} else {
		if err := bytesx.SafeWriteToBuffer(
			buffer,
			"taints = null\n",
			filepath.Dir(k.furyctlConfPath),
		); err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	}

	return nil
}

func (k *Kubernetes) addTagsToNodePool(buffer *bytes.Buffer, tags private.TypesAwsTags) error {
	if len(tags) > 0 {
		var uTags []byte

		t, err := json.Marshal(uTags)
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}

		if err := bytesx.SafeWriteToBuffer(
			buffer,
			"tags = %v\n",
			filepath.Dir(k.furyctlConfPath),
			string(t),
		); err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	} else {
		if err := bytesx.SafeWriteToBuffer(
			buffer,
			"tags = null\n",
			filepath.Dir(k.furyctlConfPath),
		); err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
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
				filepath.Dir(k.furyctlConfPath),
				strings.Join(k.furyctlConf.Spec.Kubernetes.AwsAuth.AdditionalAccounts, "\",\""),
			)
			if err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}
		}

		if len(k.furyctlConf.Spec.Kubernetes.AwsAuth.Users) > 0 {
			err = k.addAwsAuthUsers(buffer)
			if err != nil {
				return fmt.Errorf("error writing AWS Auth to Terraform vars file: %w", err)
			}
		}

		if len(k.furyctlConf.Spec.Kubernetes.AwsAuth.Roles) > 0 {
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
		filepath.Dir(k.furyctlConfPath),
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
			filepath.Dir(k.furyctlConfPath),
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
		filepath.Dir(k.furyctlConfPath),
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
		filepath.Dir(k.furyctlConfPath),
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
			filepath.Dir(k.furyctlConfPath),
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
		filepath.Dir(k.furyctlConfPath),
	)
	if err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	return nil
}

func (k *Kubernetes) addFirewallRulesToNodePool(buffer *bytes.Buffer, np private.SpecKubernetesNodePool) error {
	if np.AdditionalFirewallRules != nil {
		err := bytesx.SafeWriteToBuffer(
			buffer,
			"additional_firewall_rules = {\n",
			filepath.Dir(k.furyctlConfPath),
		)
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}

		if len(np.AdditionalFirewallRules.CidrBlocks) > 0 {
			if err := bytesx.SafeWriteToBuffer(
				buffer,
				"cidr_blocks = [\n",
				filepath.Dir(k.furyctlConfPath),
			); err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}

			if err = k.addCidrBlocksFirewallRules(buffer, np.AdditionalFirewallRules.CidrBlocks); err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}

			if err = bytesx.SafeWriteToBuffer(
				buffer,
				"]\n",
				filepath.Dir(k.furyctlConfPath),
			); err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}
		}

		if len(np.AdditionalFirewallRules.SourceSecurityGroupId) > 0 {
			if err := bytesx.SafeWriteToBuffer(
				buffer,
				"source_security_group_id = [\n",
				filepath.Dir(k.furyctlConfPath),
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
				filepath.Dir(k.furyctlConfPath),
			); err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}
		}

		if len(np.AdditionalFirewallRules.Self) > 0 {
			if err := bytesx.SafeWriteToBuffer(
				buffer,
				"self = [\n",
				filepath.Dir(k.furyctlConfPath),
			); err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}

			if err = k.addSelfFirewallRules(buffer, np.AdditionalFirewallRules.Self); err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}

			if err = bytesx.SafeWriteToBuffer(
				buffer,
				"]\n",
				filepath.Dir(k.furyctlConfPath),
			); err != nil {
				return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
			}
		}

		err = bytesx.SafeWriteToBuffer(
			buffer,
			"}\n",
			filepath.Dir(k.furyctlConfPath),
		)
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	} else {
		err := bytesx.SafeWriteToBuffer(
			buffer,
			"additional_firewall_rules = null\n",
			filepath.Dir(k.furyctlConfPath),
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
			filepath.Dir(k.furyctlConfPath),
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
			filepath.Dir(k.furyctlConfPath),
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
			filepath.Dir(k.furyctlConfPath),
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
			false,
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

	if k.furyctlConf.Spec.Kubernetes.ApiServer.PrivateAccess &&
		!k.furyctlConf.Spec.Kubernetes.ApiServer.PublicAccess &&
		k.furyctlConf.Spec.Infrastructure != nil &&
		k.furyctlConf.Spec.Infrastructure.Vpn != nil {
		return k.queryAWSDNSServer(cidr)
	}

	return nil
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
