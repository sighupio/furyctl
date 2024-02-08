// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package common

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/sighupio/fury-distribution/pkg/apis/ekscluster/v1alpha2/private"
	"github.com/sighupio/furyctl/configs"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/template"
	bytesx "github.com/sighupio/furyctl/internal/x/bytes"
	iox "github.com/sighupio/furyctl/internal/x/io"
	"github.com/sighupio/furyctl/internal/x/slices"
)

type Infrastructure struct {
	*cluster.OperationPhase

	FuryctlConf private.EksclusterKfdV1Alpha2
	ConfigPath  string
}

func (i *Infrastructure) Prepare() error {
	if err := i.CreateRootFolder(); err != nil {
		return fmt.Errorf("error creating infrastructure folder: %w", err)
	}

	if err := i.copyFromTemplate(); err != nil {
		return err
	}

	if err := i.CreateTerraformFolderStructure(); err != nil {
		return fmt.Errorf("error creating infrastructure folder structure: %w", err)
	}

	return i.createTfVars()
}

func (i *Infrastructure) copyFromTemplate() error {
	var cfg template.Config

	tmpFolder, err := os.MkdirTemp("", "furyctl-infrastructure-configs-")
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

	vpcInstallerPath := path.Join(i.Path, "..", "vendor", "installers", "eks", "modules", "vpc")
	vpnInstallerPath := path.Join(i.Path, "..", "vendor", "installers", "eks", "modules", "vpn")

	cfg.Data = map[string]map[any]any{
		"kfd": {
			"version": i.FuryctlConf.Spec.DistributionVersion,
		},
		"spec": {
			"region": i.FuryctlConf.Spec.Region,
			"tags":   i.FuryctlConf.Spec.Tags,
		},
		"kubernetes": {
			"vpcInstallerPath": vpcInstallerPath,
			"vpnInstallerPath": vpnInstallerPath,
		},
		"terraform": {
			"backend": map[string]any{
				"s3": map[string]any{
					"bucketName":           i.FuryctlConf.Spec.ToolsConfiguration.Terraform.State.S3.BucketName,
					"keyPrefix":            i.FuryctlConf.Spec.ToolsConfiguration.Terraform.State.S3.KeyPrefix,
					"region":               i.FuryctlConf.Spec.ToolsConfiguration.Terraform.State.S3.Region,
					"skipRegionValidation": i.FuryctlConf.Spec.ToolsConfiguration.Terraform.State.S3.SkipRegionValidation,
				},
			},
		},
	}

	err = i.CopyFromTemplate(
		cfg,
		"infrastructure",
		tmpFolder,
		targetTfDir,
		i.ConfigPath,
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
	vpcEnabled := i.FuryctlConf.Spec.Infrastructure.Vpc != nil

	if err := bytesx.SafeWriteToBuffer(
		buffer,
		"vpc_enabled = %v\n",
		filepath.Dir(i.ConfigPath),
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
		filepath.Dir(i.ConfigPath),
		i.FuryctlConf.Metadata.Name,
	); err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	if err := bytesx.SafeWriteToBuffer(
		buffer,
		"cidr = \"%v\"\n",
		filepath.Dir(i.ConfigPath),
		i.FuryctlConf.Spec.Infrastructure.Vpc.Network.Cidr,
	); err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	publicSubnetworkCidrs := make([]string, len(i.FuryctlConf.Spec.Infrastructure.Vpc.Network.SubnetsCidrs.Public))

	for i, cidr := range i.FuryctlConf.Spec.Infrastructure.Vpc.Network.SubnetsCidrs.Public {
		publicSubnetworkCidrs[i] = fmt.Sprintf("\"%v\"", cidr)
	}

	privateSubnetworkCidrs := make([]string, len(i.FuryctlConf.Spec.Infrastructure.Vpc.Network.SubnetsCidrs.Private))

	for i, cidr := range i.FuryctlConf.Spec.Infrastructure.Vpc.Network.SubnetsCidrs.Private {
		privateSubnetworkCidrs[i] = fmt.Sprintf("\"%v\"", cidr)
	}

	if err := bytesx.SafeWriteToBuffer(
		buffer,
		"vpc_public_subnetwork_cidrs = [%v]\n",
		filepath.Dir(i.ConfigPath),
		strings.Join(publicSubnetworkCidrs, ","),
	); err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	if err := bytesx.SafeWriteToBuffer(
		buffer,
		"vpc_private_subnetwork_cidrs = [%v]\n",
		filepath.Dir(i.ConfigPath),
		strings.Join(privateSubnetworkCidrs, ","),
	); err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	return nil
}

func (i *Infrastructure) addVpnDataToTfVars(buffer *bytes.Buffer) error {
	vpnEnabled := i.FuryctlConf.Spec.Infrastructure.Vpn != nil &&
		(i.FuryctlConf.Spec.Infrastructure.Vpn.Instances == nil ||
			(i.FuryctlConf.Spec.Infrastructure.Vpn.Instances != nil &&
				*i.FuryctlConf.Spec.Infrastructure.Vpn.Instances > 0))

	if err := bytesx.SafeWriteToBuffer(
		buffer,
		"vpn_enabled = %v\n",
		filepath.Dir(i.ConfigPath),
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
		filepath.Dir(i.ConfigPath),
		i.FuryctlConf.Spec.Infrastructure.Vpn.VpnClientsSubnetCidr,
	); err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	if i.FuryctlConf.Spec.Infrastructure.Vpn.Instances != nil {
		err := bytesx.SafeWriteToBuffer(
			buffer,
			"vpn_instances = %v\n",
			filepath.Dir(i.ConfigPath),
			*i.FuryctlConf.Spec.Infrastructure.Vpn.Instances,
		)
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	}

	if i.FuryctlConf.Spec.Infrastructure.Vpn.Port != nil && *i.FuryctlConf.Spec.Infrastructure.Vpn.Port != 0 {
		err := bytesx.SafeWriteToBuffer(
			buffer,
			"vpn_port = %v\n",
			filepath.Dir(i.ConfigPath),
			*i.FuryctlConf.Spec.Infrastructure.Vpn.Port,
		)
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	}

	if i.FuryctlConf.Spec.Infrastructure.Vpn.InstanceType != nil &&
		*i.FuryctlConf.Spec.Infrastructure.Vpn.InstanceType != "" {
		err := bytesx.SafeWriteToBuffer(
			buffer,
			"vpn_instance_type = \"%v\"\n",
			filepath.Dir(i.ConfigPath),
			*i.FuryctlConf.Spec.Infrastructure.Vpn.InstanceType,
		)
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	}

	if i.FuryctlConf.Spec.Infrastructure.Vpn.DiskSize != nil &&
		*i.FuryctlConf.Spec.Infrastructure.Vpn.DiskSize != 0 {
		err := bytesx.SafeWriteToBuffer(
			buffer,
			"vpn_instance_disk_size = %v\n",
			filepath.Dir(i.ConfigPath),
			*i.FuryctlConf.Spec.Infrastructure.Vpn.DiskSize,
		)
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	}

	if i.FuryctlConf.Spec.Infrastructure.Vpn.OperatorName != nil &&
		*i.FuryctlConf.Spec.Infrastructure.Vpn.OperatorName != "" {
		err := bytesx.SafeWriteToBuffer(
			buffer,
			"vpn_operator_name = \"%v\"\n",
			filepath.Dir(i.ConfigPath),
			*i.FuryctlConf.Spec.Infrastructure.Vpn.OperatorName,
		)
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	}

	if i.FuryctlConf.Spec.Infrastructure.Vpn.DhParamsBits != nil &&
		*i.FuryctlConf.Spec.Infrastructure.Vpn.DhParamsBits != 0 {
		err := bytesx.SafeWriteToBuffer(
			buffer,
			"vpn_dhparams_bits = %v\n",
			filepath.Dir(i.ConfigPath),
			*i.FuryctlConf.Spec.Infrastructure.Vpn.DhParamsBits,
		)
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	}

	if i.FuryctlConf.Spec.Infrastructure.Vpn.BucketNamePrefix != nil &&
		*i.FuryctlConf.Spec.Infrastructure.Vpn.BucketNamePrefix != "" {
		err := bytesx.SafeWriteToBuffer(
			buffer,
			"vpn_bucket_name_prefix = \"%v\"\n",
			filepath.Dir(i.ConfigPath),
			*i.FuryctlConf.Spec.Infrastructure.Vpn.BucketNamePrefix,
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
	if len(i.FuryctlConf.Spec.Infrastructure.Vpn.Ssh.AllowedFromCidrs) != 0 {
		uniqCidrs := slices.Uniq(i.FuryctlConf.Spec.Infrastructure.Vpn.Ssh.AllowedFromCidrs)

		allowedCidrs := make([]string, len(uniqCidrs))

		for i, cidr := range uniqCidrs {
			allowedCidrs[i] = fmt.Sprintf("\"%v\"", cidr)
		}

		err := bytesx.SafeWriteToBuffer(
			buffer,
			"vpn_operator_cidrs = [%v]\n",
			filepath.Dir(i.ConfigPath),
			strings.Join(allowedCidrs, ","),
		)
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	}

	if len(i.FuryctlConf.Spec.Infrastructure.Vpn.Ssh.GithubUsersName) != 0 {
		githubUsers := make([]string, len(i.FuryctlConf.Spec.Infrastructure.Vpn.Ssh.GithubUsersName))

		for i, gu := range i.FuryctlConf.Spec.Infrastructure.Vpn.Ssh.GithubUsersName {
			githubUsers[i] = fmt.Sprintf("\"%v\"", gu)
		}

		err := bytesx.SafeWriteToBuffer(
			buffer,
			"vpn_ssh_users = [%v]\n",
			filepath.Dir(i.ConfigPath),
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
