// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package create

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
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
	"github.com/sighupio/furyctl/internal/tool/furyagent"
	"github.com/sighupio/furyctl/internal/tool/openvpn"
	"github.com/sighupio/furyctl/internal/tool/terraform"
	bytesx "github.com/sighupio/furyctl/internal/x/bytes"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
	osx "github.com/sighupio/furyctl/internal/x/os"
)

const SErrWrapWithStr = "%w: %s"

var (
	ErrVpcIDNotFound = errors.New("vpc_id not found in infra output")
	ErrVpcIDFromOut  = errors.New("cannot read vpc_id from infrastructure's output.json")
	ErrWritingTfVars = errors.New("error writing terraform variables file")
)

type Infrastructure struct {
	*cluster.OperationPhase
	furyctlConf schema.EksclusterKfdV1Alpha2
	kfdManifest config.KFD
	tfRunner    *terraform.Runner
	faRunner    *furyagent.Runner
	ovRunner    *openvpn.Runner
	dryRun      bool
}

func NewInfrastructure(
	furyctlConf schema.EksclusterKfdV1Alpha2,
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
		faRunner: furyagent.NewRunner(executor, furyagent.Paths{
			Furyagent: path.Join(paths.BinPath, "furyagent", kfdManifest.Tools.Common.Furyagent.Version, "furyagent"),
			WorkDir:   phase.SecretsPath,
		}),
		ovRunner: openvpn.NewRunner(executor, openvpn.Paths{
			WorkDir: phase.SecretsPath,
			Openvpn: "openvpn",
		}),
		dryRun: dryRun,
	}, nil
}

func (i *Infrastructure) Exec(opts []cluster.OperationPhaseOption) error {
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

	if _, err := i.ovRunner.Version(); err != nil {
		return fmt.Errorf("can't get tool version: %w", err)
	}

	if err := i.createTfVars(); err != nil {
		return err
	}

	if err := i.tfRunner.Init(); err != nil {
		return fmt.Errorf("error running terraform init: %w", err)
	}

	if err := i.tfRunner.Plan(timestamp); err != nil {
		return fmt.Errorf("error running terraform plan: %w", err)
	}

	if i.dryRun {
		return nil
	}

	logrus.Info("Creating cloud resources, this could take a while...")

	if _, err := i.tfRunner.Apply(timestamp); err != nil {
		return fmt.Errorf("cannot create cloud resources: %w", err)
	}

	if i.isVpnConfigured() {
		clientName, err := i.generateClientName()
		if err != nil {
			return err
		}

		if err := i.faRunner.ConfigOpenvpnClient(clientName); err != nil {
			return fmt.Errorf("error configuring openvpn client: %w", err)
		}

		for _, opt := range opts {
			if strings.ToLower(opt.Name) == cluster.OperationPhaseOptionVPNAutoConnect {
				autoConnect, ok := opt.Value.(bool)
				if autoConnect && ok {
					connectMsg := "Connecting to VPN"

					isRoot, err := osx.IsRoot()
					if err != nil {
						return fmt.Errorf("error while checking if user is root: %w", err)
					}

					if !isRoot {
						connectMsg = fmt.Sprintf("%s, you will be asked for your SUDO password", connectMsg)
					}

					logrus.Infof("%s...", connectMsg)

					if err := i.ovRunner.Connect(clientName); err != nil {
						return fmt.Errorf("error connecting to VPN: %w", err)
					}
				}
			}
		}

		if err := i.copyOpenvpnToWorkDir(); err != nil {
			return fmt.Errorf("error copying openvpn file to workdir: %w", err)
		}
	}

	return nil
}

func (i *Infrastructure) isVpnConfigured() bool {
	vpn := i.furyctlConf.Spec.Infrastructure.Vpc.Vpn
	if vpn == nil {
		return false
	}

	instances := i.furyctlConf.Spec.Infrastructure.Vpc.Vpn.Instances
	if instances == nil {
		return true
	}

	return *instances > 0
}

func (i *Infrastructure) generateClientName() (string, error) {
	whoamiResp, err := exec.Command("whoami").Output()
	if err != nil {
		return "", fmt.Errorf("error getting current user: %w", err)
	}

	whoami := strings.TrimSpace(string(whoamiResp))

	return fmt.Sprintf("%s-%s", i.furyctlConf.Metadata.Name, whoami), nil
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

	cfg.Data = map[string]map[any]any{
		"kubernetes": {
			"eks": i.kfdManifest.Kubernetes.Eks,
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

func (i *Infrastructure) copyOpenvpnToWorkDir() error {
	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("error getting current dir: %w", err)
	}

	ovpnFileName, err := i.generateClientName()
	if err != nil {
		return err
	}

	ovpnFileName = fmt.Sprintf("%s.ovpn", ovpnFileName)

	ovpnPath, err := filepath.Abs(path.Join(i.SecretsPath, ovpnFileName))
	if err != nil {
		return fmt.Errorf("error getting ovpn absolute path: %w", err)
	}

	ovpnFile, err := os.ReadFile(ovpnPath)
	if err != nil {
		return fmt.Errorf("error reading ovpn file: %w", err)
	}

	err = os.WriteFile(path.Join(currentDir, ovpnFileName), ovpnFile, iox.FullRWPermAccess)
	if err != nil {
		return fmt.Errorf("error writing ovpn file: %w", err)
	}

	return nil
}

func (i *Infrastructure) createTfVars() error {
	var buffer bytes.Buffer

	err := bytesx.SafeWriteToBuffer(&buffer, "name = \"%v\"\n", i.furyctlConf.Metadata.Name)
	if err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	err = bytesx.SafeWriteToBuffer(
		&buffer,
		"network_cidr = \"%v\"\n",
		i.furyctlConf.Spec.Infrastructure.Vpc.Network.Cidr,
	)
	if err != nil {
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

	err = bytesx.SafeWriteToBuffer(&buffer,
		"public_subnetwork_cidrs = [%v]\n",
		strings.Join(publicSubnetworkCidrs, ","),
	)
	if err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	err = bytesx.SafeWriteToBuffer(&buffer,
		"private_subnetwork_cidrs = [%v]\n",
		strings.Join(privateSubnetworkCidrs, ","),
	)
	if err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	if i.furyctlConf.Spec.Infrastructure.Vpc.Vpn != nil {
		err = i.addVpnDataToTfVars(&buffer)
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	}

	return i.writeTfVars(buffer)
}

func (i *Infrastructure) addVpnDataToTfVars(buffer *bytes.Buffer) error {
	err := bytesx.SafeWriteToBuffer(
		buffer,
		"vpn_subnetwork_cidr = \"%v\"\n",
		i.furyctlConf.Spec.Infrastructure.Vpc.Vpn.VpnClientsSubnetCidr,
	)
	if err != nil {
		return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
	}

	if i.furyctlConf.Spec.Infrastructure.Vpc.Vpn.Instances != nil {
		err = bytesx.SafeWriteToBuffer(
			buffer,
			"vpn_instances = %v\n",
			*i.furyctlConf.Spec.Infrastructure.Vpc.Vpn.Instances,
		)
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	}

	if i.furyctlConf.Spec.Infrastructure.Vpc.Vpn.Port != nil && *i.furyctlConf.Spec.Infrastructure.Vpc.Vpn.Port != 0 {
		err = bytesx.SafeWriteToBuffer(
			buffer,
			"vpn_port = %v\n",
			*i.furyctlConf.Spec.Infrastructure.Vpc.Vpn.Port,
		)
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	}

	if i.furyctlConf.Spec.Infrastructure.Vpc.Vpn.InstanceType != nil &&
		*i.furyctlConf.Spec.Infrastructure.Vpc.Vpn.InstanceType != "" {
		err = bytesx.SafeWriteToBuffer(
			buffer,
			"vpn_instance_type = \"%v\"\n",
			*i.furyctlConf.Spec.Infrastructure.Vpc.Vpn.InstanceType,
		)
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	}

	if i.furyctlConf.Spec.Infrastructure.Vpc.Vpn.DiskSize != nil &&
		*i.furyctlConf.Spec.Infrastructure.Vpc.Vpn.DiskSize != 0 {
		err = bytesx.SafeWriteToBuffer(
			buffer,
			"vpn_instance_disk_size = %v\n",
			*i.furyctlConf.Spec.Infrastructure.Vpc.Vpn.DiskSize,
		)
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	}

	if i.furyctlConf.Spec.Infrastructure.Vpc.Vpn.OperatorName != nil &&
		*i.furyctlConf.Spec.Infrastructure.Vpc.Vpn.OperatorName != "" {
		err = bytesx.SafeWriteToBuffer(
			buffer,
			"vpn_operator_name = \"%v\"\n",
			*i.furyctlConf.Spec.Infrastructure.Vpc.Vpn.OperatorName,
		)
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	}

	if i.furyctlConf.Spec.Infrastructure.Vpc.Vpn.DhParamsBits != nil &&
		*i.furyctlConf.Spec.Infrastructure.Vpc.Vpn.DhParamsBits != 0 {
		err = bytesx.SafeWriteToBuffer(
			buffer,
			"vpn_dhparams_bits = %v\n",
			*i.furyctlConf.Spec.Infrastructure.Vpc.Vpn.DhParamsBits,
		)
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	}

	if len(i.furyctlConf.Spec.Infrastructure.Vpc.Vpn.Ssh.AllowedFromCidrs) != 0 {
		allowedCidrs := make([]string, len(i.furyctlConf.Spec.Infrastructure.Vpc.Vpn.Ssh.AllowedFromCidrs))

		for i, cidr := range i.furyctlConf.Spec.Infrastructure.Vpc.Vpn.Ssh.AllowedFromCidrs {
			allowedCidrs[i] = fmt.Sprintf("\"%v\"", cidr)
		}

		err = bytesx.SafeWriteToBuffer(
			buffer,
			"vpn_operator_cidrs = [%v]\n",
			strings.Join(allowedCidrs, ","),
		)
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, ErrWritingTfVars, err)
		}
	}

	if len(i.furyctlConf.Spec.Infrastructure.Vpc.Vpn.Ssh.GithubUsersName) != 0 {
		githubUsers := make([]string, len(i.furyctlConf.Spec.Infrastructure.Vpc.Vpn.Ssh.GithubUsersName))

		for i, gu := range i.furyctlConf.Spec.Infrastructure.Vpc.Vpn.Ssh.GithubUsersName {
			githubUsers[i] = fmt.Sprintf("\"%v\"", gu)
		}

		err = bytesx.SafeWriteToBuffer(
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
