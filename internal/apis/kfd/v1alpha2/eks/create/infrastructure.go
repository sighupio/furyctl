// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package create

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/fury-distribution/pkg/apis/ekscluster/v1alpha2/private"
	"github.com/sighupio/furyctl/configs"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/eks"
	"github.com/sighupio/furyctl/internal/parser"
	"github.com/sighupio/furyctl/internal/template"
	"github.com/sighupio/furyctl/internal/tool/terraform"
	"github.com/sighupio/furyctl/internal/upgrade"
	bytesx "github.com/sighupio/furyctl/internal/x/bytes"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
	"github.com/sighupio/furyctl/internal/x/slices"
)

const SErrWrapWithStr = "%w: %s"

var (
	ErrVpcIDNotFound = errors.New("vpc_id not found in infra output")
	ErrVpcIDFromOut  = errors.New("cannot read vpc_id from infrastructure's output.json")
	ErrWritingTfVars = errors.New("error writing terraform variables file")
	ErrAbortedByUser = errors.New("aborted by user")
)

type Infrastructure struct {
	*cluster.OperationPhase
	furyctlConf     private.EksclusterKfdV1Alpha2
	kfdManifest     config.KFD
	furyctlConfPath string
	tfRunner        *terraform.Runner
	dryRun          bool
	upgrade         *upgrade.Upgrade
}

func NewInfrastructure(
	furyctlConf private.EksclusterKfdV1Alpha2,
	kfdManifest config.KFD,
	paths cluster.CreatorPaths,
	dryRun bool,
	upgrade *upgrade.Upgrade,
) (*Infrastructure, error) {
	infraDir := path.Join(paths.WorkDir, cluster.OperationPhaseInfrastructure)

	phase, err := cluster.NewOperationPhase(infraDir, kfdManifest.Tools, paths.BinPath)
	if err != nil {
		return nil, fmt.Errorf("error creating infrastructure phase: %w", err)
	}

	executor := execx.NewStdExecutor()

	return &Infrastructure{
		OperationPhase:  phase,
		furyctlConf:     furyctlConf,
		kfdManifest:     kfdManifest,
		furyctlConfPath: paths.ConfigPath,
		tfRunner: terraform.NewRunner(
			executor,
			terraform.Paths{
				Logs:      phase.TerraformLogsPath,
				Outputs:   phase.TerraformOutputsPath,
				WorkDir:   path.Join(phase.Path, "terraform"),
				Plan:      phase.TerraformPlanPath,
				Terraform: phase.TerraformPath,
			},
		),
		dryRun:  dryRun,
		upgrade: upgrade,
	}, nil
}

func (i *Infrastructure) Exec() error {
	logrus.Info("Creating infrastructure...")

	logrus.Debug("Create: running infrastructure phase...")

	timestamp := time.Now().Unix()

	if err := i.CreateFolder(); err != nil {
		return fmt.Errorf("error creating infrastructure folder: %w", err)
	}

	if err := i.copyFromTemplate(); err != nil {
		return err
	}

	if err := i.CreateFolderStructure(); err != nil {
		return fmt.Errorf("error creating infrastructure folder structure: %w", err)
	}

	if err := i.createTfVars(); err != nil {
		return err
	}

	if err := i.tfRunner.Init(); err != nil {
		return fmt.Errorf("error running terraform init: %w", err)
	}

	if !i.dryRun {
		if err := i.upgrade.Exec(i.Path, "pre-infrastructure"); err != nil {
			return fmt.Errorf("error running upgrade: %w", err)
		}
	}

	plan, err := i.tfRunner.Plan(timestamp)
	if err != nil {
		return fmt.Errorf("error running terraform plan: %w", err)
	}

	if i.dryRun {
		return nil
	}

	tfParser := parser.NewTfPlanParser(string(plan))

	parsedPlan := tfParser.Parse()

	eksInf := eks.NewInfra()

	criticalResources := slices.Intersection(eksInf.GetCriticalTFResourceTypes(), parsedPlan.Destroy)

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

	logrus.Warn("Creating cloud resources, this could take a while...")

	if err := i.tfRunner.Apply(timestamp); err != nil {
		return fmt.Errorf("cannot create cloud resources: %w", err)
	}

	if _, err := i.tfRunner.Output(); err != nil {
		return fmt.Errorf("error getting terraform output: %w", err)
	}

	// Run upgrade script if needed.
	if err := i.upgrade.Exec(i.Path, "post-infrastructure"); err != nil {
		return fmt.Errorf("error running upgrade: %w", err)
	}

	return nil
}

func (i *Infrastructure) Stop() error {
	logrus.Debug("Stopping terraform...")

	if err := i.tfRunner.Stop(); err != nil {
		return fmt.Errorf("error stopping terraform: %w", err)
	}

	return nil
}

func (i *Infrastructure) copyFromTemplate() error {
	var cfg template.Config

	tmpFolder, err := os.MkdirTemp("", "furyctl-infra-configs-")
	if err != nil {
		return fmt.Errorf("error creating temp folder: %w", err)
	}

	defer os.RemoveAll(tmpFolder)

	subFS, err := fs.Sub(configs.Tpl, path.Join("provisioners", "bootstrap", "aws"))
	if err != nil {
		return fmt.Errorf("error getting subfs: %w", err)
	}

	if err = iox.CopyRecursive(subFS, tmpFolder); err != nil {
		return fmt.Errorf("error copying template files: %w", err)
	}

	targetTfDir := path.Join(i.Path, "terraform")
	prefix := "infra"

	vpcInstallerPath := path.Join(i.Path, "..", "vendor", "installers", "eks", "modules", "vpc")
	vpnInstallerPath := path.Join(i.Path, "..", "vendor", "installers", "eks", "modules", "vpn")

	cfg.Data = map[string]map[any]any{
		"spec": {
			"region": i.furyctlConf.Spec.Region,
			"tags":   i.furyctlConf.Spec.Tags,
		},
		"kubernetes": {
			"vpcInstallerPath": vpcInstallerPath,
			"vpnInstallerPath": vpnInstallerPath,
		},
		"terraform": {
			"backend": map[string]any{
				"s3": map[string]any{
					"bucketName":           i.furyctlConf.Spec.ToolsConfiguration.Terraform.State.S3.BucketName,
					"keyPrefix":            i.furyctlConf.Spec.ToolsConfiguration.Terraform.State.S3.KeyPrefix,
					"region":               i.furyctlConf.Spec.ToolsConfiguration.Terraform.State.S3.Region,
					"skipRegionValidation": i.furyctlConf.Spec.ToolsConfiguration.Terraform.State.S3.SkipRegionValidation,
				},
			},
		},
	}

	err = i.OperationPhase.CopyFromTemplate(
		cfg,
		prefix,
		tmpFolder,
		targetTfDir,
		i.furyctlConfPath,
	)
	if err != nil {
		return fmt.Errorf("error generating from template files: %w", err)
	}

	return nil
}

func (i *Infrastructure) createTfVars() error {
	var buffer bytes.Buffer

	if err := i.addVpcDataToTfVars(&buffer); err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	if err := i.addVpnDataToTfVars(&buffer); err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	return i.writeTfVars(buffer)
}

func (i *Infrastructure) addVpcDataToTfVars(buffer *bytes.Buffer) error {
	vpcEnabled := i.furyctlConf.Spec.Infrastructure.Vpc != nil

	if err := bytesx.SafeWriteToBuffer(
		buffer,
		"vpc_enabled = %v\n",
		filepath.Dir(i.furyctlConfPath),
		vpcEnabled,
	); err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	if !vpcEnabled {
		return nil
	}

	if err := bytesx.SafeWriteToBuffer(
		buffer,
		"name = \"%v\"\n",
		filepath.Dir(i.furyctlConfPath),
		i.furyctlConf.Metadata.Name,
	); err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	if err := bytesx.SafeWriteToBuffer(
		buffer,
		"cidr = \"%v\"\n",
		filepath.Dir(i.furyctlConfPath),
		i.furyctlConf.Spec.Infrastructure.Vpc.Network.Cidr,
	); err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	publicSubnetworkCidrs := make([]string, len(i.furyctlConf.Spec.Infrastructure.Vpc.Network.SubnetsCidrs.Public))

	for i, cidr := range i.furyctlConf.Spec.Infrastructure.Vpc.Network.SubnetsCidrs.Public {
		publicSubnetworkCidrs[i] = fmt.Sprintf("\"%v\"", cidr)
	}

	privateSubnetworkCidrs := make([]string, len(i.furyctlConf.Spec.Infrastructure.Vpc.Network.SubnetsCidrs.Private))

	for i, cidr := range i.furyctlConf.Spec.Infrastructure.Vpc.Network.SubnetsCidrs.Private {
		privateSubnetworkCidrs[i] = fmt.Sprintf("\"%v\"", cidr)
	}

	if err := bytesx.SafeWriteToBuffer(
		buffer,
		"vpc_public_subnetwork_cidrs = [%v]\n",
		filepath.Dir(i.furyctlConfPath),
		strings.Join(publicSubnetworkCidrs, ","),
	); err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	if err := bytesx.SafeWriteToBuffer(
		buffer,
		"vpc_private_subnetwork_cidrs = [%v]\n",
		filepath.Dir(i.furyctlConfPath),
		strings.Join(privateSubnetworkCidrs, ","),
	); err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	return nil
}

func (i *Infrastructure) addVpnDataToTfVars(buffer *bytes.Buffer) error {
	vpnEnabled := i.furyctlConf.Spec.Infrastructure.Vpn != nil &&
		(i.furyctlConf.Spec.Infrastructure.Vpn.Instances == nil ||
			(i.furyctlConf.Spec.Infrastructure.Vpn.Instances != nil &&
				*i.furyctlConf.Spec.Infrastructure.Vpn.Instances > 0))

	if err := bytesx.SafeWriteToBuffer(
		buffer,
		"vpn_enabled = %v\n",
		filepath.Dir(i.furyctlConfPath),
		vpnEnabled,
	); err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	if !vpnEnabled {
		return nil
	}

	if err := bytesx.SafeWriteToBuffer(
		buffer,
		"vpn_subnetwork_cidr = \"%v\"\n",
		filepath.Dir(i.furyctlConfPath),
		i.furyctlConf.Spec.Infrastructure.Vpn.VpnClientsSubnetCidr,
	); err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	if i.furyctlConf.Spec.Infrastructure.Vpn.Instances != nil {
		err := bytesx.SafeWriteToBuffer(
			buffer,
			"vpn_instances = %v\n",
			filepath.Dir(i.furyctlConfPath),
			*i.furyctlConf.Spec.Infrastructure.Vpn.Instances,
		)
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	}

	if i.furyctlConf.Spec.Infrastructure.Vpn.Port != nil && *i.furyctlConf.Spec.Infrastructure.Vpn.Port != 0 {
		err := bytesx.SafeWriteToBuffer(
			buffer,
			"vpn_port = %v\n",
			filepath.Dir(i.furyctlConfPath),
			*i.furyctlConf.Spec.Infrastructure.Vpn.Port,
		)
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	}

	if i.furyctlConf.Spec.Infrastructure.Vpn.InstanceType != nil &&
		*i.furyctlConf.Spec.Infrastructure.Vpn.InstanceType != "" {
		err := bytesx.SafeWriteToBuffer(
			buffer,
			"vpn_instance_type = \"%v\"\n",
			filepath.Dir(i.furyctlConfPath),
			*i.furyctlConf.Spec.Infrastructure.Vpn.InstanceType,
		)
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	}

	if i.furyctlConf.Spec.Infrastructure.Vpn.DiskSize != nil &&
		*i.furyctlConf.Spec.Infrastructure.Vpn.DiskSize != 0 {
		err := bytesx.SafeWriteToBuffer(
			buffer,
			"vpn_instance_disk_size = %v\n",
			filepath.Dir(i.furyctlConfPath),
			*i.furyctlConf.Spec.Infrastructure.Vpn.DiskSize,
		)
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	}

	if i.furyctlConf.Spec.Infrastructure.Vpn.OperatorName != nil &&
		*i.furyctlConf.Spec.Infrastructure.Vpn.OperatorName != "" {
		err := bytesx.SafeWriteToBuffer(
			buffer,
			"vpn_operator_name = \"%v\"\n",
			filepath.Dir(i.furyctlConfPath),
			*i.furyctlConf.Spec.Infrastructure.Vpn.OperatorName,
		)
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	}

	if i.furyctlConf.Spec.Infrastructure.Vpn.DhParamsBits != nil &&
		*i.furyctlConf.Spec.Infrastructure.Vpn.DhParamsBits != 0 {
		err := bytesx.SafeWriteToBuffer(
			buffer,
			"vpn_dhparams_bits = %v\n",
			filepath.Dir(i.furyctlConfPath),
			*i.furyctlConf.Spec.Infrastructure.Vpn.DhParamsBits,
		)
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	}

	if i.furyctlConf.Spec.Infrastructure.Vpn.BucketNamePrefix != nil &&
		*i.furyctlConf.Spec.Infrastructure.Vpn.BucketNamePrefix != "" {
		err := bytesx.SafeWriteToBuffer(
			buffer,
			"vpn_bucket_name_prefix = \"%v\"\n",
			filepath.Dir(i.furyctlConfPath),
			*i.furyctlConf.Spec.Infrastructure.Vpn.BucketNamePrefix,
		)
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	}

	if err := i.addVpnSSHDataToTfVars(buffer); err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	return nil
}

func (i *Infrastructure) addVpnSSHDataToTfVars(buffer *bytes.Buffer) error {
	if len(i.furyctlConf.Spec.Infrastructure.Vpn.Ssh.AllowedFromCidrs) != 0 {
		uniqCidrs := slices.Uniq(i.furyctlConf.Spec.Infrastructure.Vpn.Ssh.AllowedFromCidrs)

		allowedCidrs := make([]string, len(uniqCidrs))

		for i, cidr := range uniqCidrs {
			allowedCidrs[i] = fmt.Sprintf("\"%v\"", cidr)
		}

		err := bytesx.SafeWriteToBuffer(
			buffer,
			"vpn_operator_cidrs = [%v]\n",
			filepath.Dir(i.furyctlConfPath),
			strings.Join(allowedCidrs, ","),
		)
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	}

	if len(i.furyctlConf.Spec.Infrastructure.Vpn.Ssh.GithubUsersName) != 0 {
		githubUsers := make([]string, len(i.furyctlConf.Spec.Infrastructure.Vpn.Ssh.GithubUsersName))

		for i, gu := range i.furyctlConf.Spec.Infrastructure.Vpn.Ssh.GithubUsersName {
			githubUsers[i] = fmt.Sprintf("\"%v\"", gu)
		}

		err := bytesx.SafeWriteToBuffer(
			buffer,
			"vpn_ssh_users = [%v]\n",
			filepath.Dir(i.furyctlConfPath),
			strings.Join(githubUsers, ","),
		)
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	}

	return nil
}

func (i *Infrastructure) writeTfVars(buffer bytes.Buffer) error {
	targetTfVars := path.Join(i.Path, "terraform", "main.auto.tfvars")

	err := os.WriteFile(targetTfVars, buffer.Bytes(), iox.FullRWPermAccess)
	if err != nil {
		return fmt.Errorf("error writing terraform vars: %w", err)
	}

	return nil
}
