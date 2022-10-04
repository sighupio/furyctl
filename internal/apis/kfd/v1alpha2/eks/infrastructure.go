// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eks

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/sighupio/fury-distribution/pkg/config"
	"github.com/sighupio/fury-distribution/pkg/schema"
	"github.com/sighupio/furyctl/configs"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/template"
	"github.com/sighupio/furyctl/internal/tool/furyagent"
	"github.com/sighupio/furyctl/internal/tool/openvpn"
	"github.com/sighupio/furyctl/internal/tool/terraform"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
)

type Infrastructure struct {
	*cluster.CreationPhase
	furyctlConf schema.EksclusterKfdV1Alpha2
	kfdManifest config.KFD
	tfRunner    *terraform.Runner
	faRunner    *furyagent.Runner
	ovRunner    *openvpn.Runner
}

func NewInfrastructure(furyctlConf schema.EksclusterKfdV1Alpha2, kfdManifest config.KFD) (*Infrastructure, error) {
	phase, err := cluster.NewCreationPhase(".infrastructure")
	if err != nil {
		return nil, err
	}

	executor := execx.NewStdExecutor()

	return &Infrastructure{
		CreationPhase: phase,
		furyctlConf:   furyctlConf,
		kfdManifest:   kfdManifest,
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
			Furyagent: path.Join(phase.VendorPath, "bin", "furyagent"),
			WorkDir:   phase.SecretsPath,
		}),
		ovRunner: openvpn.NewRunner(executor, openvpn.Paths{
			WorkDir: phase.SecretsPath,
			Openvpn: "openvpn",
		}),
	}, nil
}

func (i *Infrastructure) Exec(dryRun bool, opts []cluster.CreationPhaseOption) error {
	timestamp := time.Now().Unix()

	if err := i.CreateFolder(); err != nil {
		return err
	}

	if err := i.copyFromTemplate(i.kfdManifest); err != nil {
		return err
	}

	if err := i.CreateFolderStructure(); err != nil {
		return err
	}

	if err := i.createTfVars(); err != nil {
		return err
	}

	if err := i.tfRunner.Init(); err != nil {
		return err
	}

	if err := i.tfRunner.Plan(timestamp); err != nil {
		return err
	}

	if dryRun {
		return nil
	}

	if _, err := i.tfRunner.Apply(timestamp); err != nil {
		return err
	}

	if i.isVpnConfigured() {
		clientName, err := i.generateClientName()
		if err != nil {
			return err
		}

		if err := i.faRunner.ConfigOpenvpnClient(clientName); err != nil {
			return err
		}

		for _, opt := range opts {
			switch strings.ToLower(opt.Name) {
			case cluster.CreationPhaseOptionVPNAutoConnect:
				if err := i.ovRunner.Connect(clientName); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (i *Infrastructure) isVpnConfigured() bool {
	return i.furyctlConf.Spec.Infrastructure.Vpc.Vpn != nil && i.furyctlConf.Spec.Infrastructure.Vpc.Vpn.Instances > 0
}

func (i *Infrastructure) generateClientName() (string, error) {
	whoamiResp, err := exec.Command("whoami").Output()
	if err != nil {
		return "", err
	}

	whoami := strings.TrimSpace(string(whoamiResp))

	return fmt.Sprintf("%s-%s", i.furyctlConf.Metadata.Name, whoami), nil
}

func (i *Infrastructure) copyFromTemplate(kfdManifest config.KFD) error {
	var cfg template.Config

	tmpFolder, err := os.MkdirTemp("", "furyctl-infra-configs-")
	if err != nil {
		return err
	}

	defer os.RemoveAll(tmpFolder)

	subFS, err := fs.Sub(configs.Tpl, path.Join("provisioners", "bootstrap", "aws"))
	if err != nil {
		return err
	}

	if err = iox.CopyRecursive(subFS, tmpFolder); err != nil {
		return err
	}

	targetTfDir := path.Join(i.Path, "terraform")
	prefix := "infra"

	cfg.Data = map[string]map[any]any{
		"kubernetes": {
			"eks": kfdManifest.Kubernetes.Eks,
		},
	}

	return i.CreationPhase.CopyFromTemplate(
		cfg,
		prefix,
		tmpFolder,
		targetTfDir,
	)
}

func (i *Infrastructure) createTfVars() error {
	var buffer bytes.Buffer

	buffer.WriteString(fmt.Sprintf("name = \"%v\"\n", i.furyctlConf.Metadata.Name))
	buffer.WriteString(fmt.Sprintf(
		"network_cidr = \"%v\"\n",
		i.furyctlConf.Spec.Infrastructure.Vpc.Network.Cidr,
	))

	publicSubnetworkCidrs := make([]string, len(i.furyctlConf.Spec.Infrastructure.Vpc.Network.SubnetsCidrs.Public))

	for i, cidr := range i.furyctlConf.Spec.Infrastructure.Vpc.Network.SubnetsCidrs.Public {
		publicSubnetworkCidrs[i] = fmt.Sprintf("\"%v\"", cidr)
	}

	privateSubnetworkCidrs := make([]string, len(i.furyctlConf.Spec.Infrastructure.Vpc.Network.SubnetsCidrs.Private))

	for i, cidr := range i.furyctlConf.Spec.Infrastructure.Vpc.Network.SubnetsCidrs.Private {
		privateSubnetworkCidrs[i] = fmt.Sprintf("\"%v\"", cidr)
	}

	buffer.WriteString(fmt.Sprintf(
		"public_subnetwork_cidrs = [%v]\n",
		strings.Join(publicSubnetworkCidrs, ",")))

	buffer.WriteString(fmt.Sprintf(
		"private_subnetwork_cidrs = [%v]\n",
		strings.Join(privateSubnetworkCidrs, ",")))

	if i.furyctlConf.Spec.Infrastructure.Vpc.Vpn != nil {
		buffer.WriteString(
			fmt.Sprintf(
				"vpn_subnetwork_cidr = \"%v\"\n",
				i.furyctlConf.Spec.Infrastructure.Vpc.Vpn.VpnClientsSubnetCidr,
			),
		)
		buffer.WriteString(
			fmt.Sprintf(
				"vpn_instances = %v\n",
				i.furyctlConf.Spec.Infrastructure.Vpc.Vpn.Instances,
			),
		)

		if i.furyctlConf.Spec.Infrastructure.Vpc.Vpn.Port != 0 {
			buffer.WriteString(
				fmt.Sprintf(
					"vpn_port = %v\n",
					i.furyctlConf.Spec.Infrastructure.Vpc.Vpn.Port,
				),
			)
		}

		if i.furyctlConf.Spec.Infrastructure.Vpc.Vpn.InstanceType != "" {
			buffer.WriteString(
				fmt.Sprintf(
					"vpn_instance_type = \"%v\"\n",
					i.furyctlConf.Spec.Infrastructure.Vpc.Vpn.InstanceType,
				),
			)
		}

		if i.furyctlConf.Spec.Infrastructure.Vpc.Vpn.DiskSize != 0 {
			buffer.WriteString(
				fmt.Sprintf(
					"vpn_instance_disk_size = %v\n",
					i.furyctlConf.Spec.Infrastructure.Vpc.Vpn.DiskSize,
				),
			)
		}

		if i.furyctlConf.Spec.Infrastructure.Vpc.Vpn.OperatorName != "" {
			buffer.WriteString(
				fmt.Sprintf(
					"vpn_operator_name = \"%v\"\n",
					i.furyctlConf.Spec.Infrastructure.Vpc.Vpn.OperatorName,
				),
			)
		}

		if i.furyctlConf.Spec.Infrastructure.Vpc.Vpn.DhParamsBits != 0 {
			buffer.WriteString(
				fmt.Sprintf(
					"vpn_dhparams_bits = %v\n",
					i.furyctlConf.Spec.Infrastructure.Vpc.Vpn.DhParamsBits,
				),
			)
		}

		if len(i.furyctlConf.Spec.Infrastructure.Vpc.Vpn.Ssh.AllowedFromCidrs) != 0 {
			allowedCidrs := make([]string, len(i.furyctlConf.Spec.Infrastructure.Vpc.Vpn.Ssh.AllowedFromCidrs))

			for i, cidr := range i.furyctlConf.Spec.Infrastructure.Vpc.Vpn.Ssh.AllowedFromCidrs {
				allowedCidrs[i] = fmt.Sprintf("\"%v\"", cidr)
			}

			buffer.WriteString(
				fmt.Sprintf(
					"vpn_operator_cidrs = [%v]\n",
					strings.Join(allowedCidrs, ","),
				),
			)
		}

		if len(i.furyctlConf.Spec.Infrastructure.Vpc.Vpn.Ssh.GithubUsersName) != 0 {
			githubUsers := make([]string, len(i.furyctlConf.Spec.Infrastructure.Vpc.Vpn.Ssh.GithubUsersName))

			for i, gu := range i.furyctlConf.Spec.Infrastructure.Vpc.Vpn.Ssh.GithubUsersName {
				githubUsers[i] = fmt.Sprintf("\"%v\"", gu)
			}

			buffer.WriteString(
				fmt.Sprintf(
					"vpn_ssh_users = [%v]\n",
					strings.Join(githubUsers, ","),
				),
			)
		}
	}

	targetTfVars := path.Join(i.Path, "terraform", "main.auto.tfvars")

	return os.WriteFile(targetTfVars, buffer.Bytes(), 0o600)
}
