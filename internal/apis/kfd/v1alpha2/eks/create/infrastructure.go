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
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/config"
	"github.com/sighupio/fury-distribution/pkg/schema/private"
	"github.com/sighupio/furyctl/configs"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/eks"
	"github.com/sighupio/furyctl/internal/parser"
	"github.com/sighupio/furyctl/internal/template"
	"github.com/sighupio/furyctl/internal/tool/terraform"
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
	furyctlConf private.EksclusterKfdV1Alpha2
	kfdManifest config.KFD
	tfRunner    *terraform.Runner
	dryRun      bool
}

func NewInfrastructure(
	furyctlConf private.EksclusterKfdV1Alpha2,
	kfdManifest config.KFD,
	paths cluster.CreatorPaths,
	dryRun bool,
) (*Infrastructure, error) {
	infraDir := path.Join(paths.WorkDir, cluster.OperationPhaseInfrastructure)

	phase, err := cluster.NewOperationPhase(infraDir, kfdManifest.Tools, paths.BinPath)
	if err != nil {
		return nil, fmt.Errorf("error creating infrastructure phase: %w", err)
	}

	executor := execx.NewStdExecutor()

	return &Infrastructure{
		OperationPhase: phase,
		furyctlConf:    furyctlConf,
		kfdManifest:    kfdManifest,
		tfRunner: terraform.NewRunner(
			executor,
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

	if _, err := i.tfRunner.Apply(timestamp); err != nil {
		return fmt.Errorf("cannot create cloud resources: %w", err)
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
					"bucketName": i.furyctlConf.Spec.ToolsConfiguration.Terraform.State.S3.BucketName,
					"keyPrefix":  i.furyctlConf.Spec.ToolsConfiguration.Terraform.State.S3.KeyPrefix,
					"region":     i.furyctlConf.Spec.ToolsConfiguration.Terraform.State.S3.Region,
				},
			},
		},
	}

	err = i.OperationPhase.CopyFromTemplate(
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

func (i *Infrastructure) createTfVars() error {
	var buffer bytes.Buffer

	if i.furyctlConf.Spec.Infrastructure.Vpc != nil {
		if err := i.addVpcDataToTfVars(&buffer); err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	}

	if i.furyctlConf.Spec.Infrastructure.Vpn != nil {
		if err := i.addVpnDataToTfVars(&buffer); err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	}

	return i.writeTfVars(buffer)
}

func (i *Infrastructure) addVpcDataToTfVars(buffer *bytes.Buffer) error {
	vpcEnabled := i.furyctlConf.Spec.Infrastructure.Vpc != nil

	if err := bytesx.SafeWriteToBuffer(buffer,
		"vpc_enabled = %v\n",
		vpcEnabled,
	); err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	if err := bytesx.SafeWriteToBuffer(buffer, "name = \"%v\"\n", i.furyctlConf.Metadata.Name); err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	if err := bytesx.SafeWriteToBuffer(
		buffer,
		"cidr = \"%v\"\n",
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

	if err := bytesx.SafeWriteToBuffer(buffer,
		"vpc_public_subnetwork_cidrs = [%v]\n",
		strings.Join(publicSubnetworkCidrs, ","),
	); err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	if err := bytesx.SafeWriteToBuffer(buffer,
		"vpc_private_subnetwork_cidrs = [%v]\n",
		strings.Join(privateSubnetworkCidrs, ","),
	); err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	return nil
}

func (i *Infrastructure) addVpnDataToTfVars(buffer *bytes.Buffer) error {
	vpnEnabled := (i.furyctlConf.Spec.Infrastructure.Vpn != nil) &&
		(i.furyctlConf.Spec.Infrastructure.Vpn.Instances == nil) ||
		(i.furyctlConf.Spec.Infrastructure.Vpn.Instances != nil) &&
			(*i.furyctlConf.Spec.Infrastructure.Vpn.Instances > 0)

	if err := bytesx.SafeWriteToBuffer(buffer, "vpn_enabled = %v\n", vpnEnabled); err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	if err := bytesx.SafeWriteToBuffer(
		buffer,
		"vpn_subnetwork_cidr = \"%v\"\n",
		i.furyctlConf.Spec.Infrastructure.Vpn.VpnClientsSubnetCidr,
	); err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	if i.furyctlConf.Spec.Infrastructure.Vpn.Instances != nil {
		err := bytesx.SafeWriteToBuffer(
			buffer,
			"vpn_instances = %v\n",
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
			*i.furyctlConf.Spec.Infrastructure.Vpn.DhParamsBits,
		)
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	}

	if len(i.furyctlConf.Spec.Infrastructure.Vpn.Ssh.AllowedFromCidrs) != 0 {
		allowedCidrs := make([]string, len(i.furyctlConf.Spec.Infrastructure.Vpn.Ssh.AllowedFromCidrs))

		for i, cidr := range i.furyctlConf.Spec.Infrastructure.Vpn.Ssh.AllowedFromCidrs {
			allowedCidrs[i] = fmt.Sprintf("\"%v\"", cidr)
		}

		err := bytesx.SafeWriteToBuffer(
			buffer,
			"vpn_operator_cidrs = [%v]\n",
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
