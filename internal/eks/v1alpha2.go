// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eks

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/schemas"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/yaml"
)

var ErrUnsupportedPhase = fmt.Errorf("unsupported phase")

type V1alpha2 struct {
	Phase          string
	KfdManifest    distribution.Manifest
	ConfigPath     string
	VpnAutoConnect bool
}

func (v *V1alpha2) Create(dryRun bool) error {
	logrus.Infof("Running phase: %s", v.Phase)

	switch v.Phase {
	case "infrastructure":
		return v.Infrastructure(dryRun)
	case "kubernetes":
		return v.Kubernetes(dryRun)
	case "distribution":
		return v.Distribution(dryRun)
	case "":
		err := v.Infrastructure(dryRun)
		if err != nil {
			return err
		}

		err = v.Kubernetes(dryRun)
		if err != nil {
			return err
		}

		return v.Distribution(dryRun)
	default:
		return ErrUnsupportedPhase
	}
}

func (v *V1alpha2) Infrastructure(dryRun bool) error {
	timestamp := time.Now().Unix()

	infra, err := NewInfrastructure()
	if err != nil {
		return err
	}

	err = infra.CreateFolder()
	if err != nil {
		return err
	}

	err = infra.CopyFromTemplate(v.KfdManifest)
	if err != nil {
		return err
	}

	err = infra.CreateFolderStructure()
	if err != nil {
		return err
	}

	furyFile, err := v.GetConfigFile()
	if err != nil {
		return err
	}

	err = v.CreateTfVars(furyFile, infra.Path)
	if err != nil {
		return err
	}

	err = infra.TerraformInit()
	if err != nil {
		return err
	}

	err = infra.TerraformPlan(timestamp)
	if err != nil {
		return err
	}

	if !dryRun {
		err = infra.TerraformApply(timestamp)
		if err != nil {
			return err
		}

		if furyFile.Spec.Infrastructure.Vpc.Vpn != nil && furyFile.Spec.Infrastructure.Vpc.Vpn.Instances > 0 {
			clientName := furyFile.Metadata.Name

			whoamiResp, err := exec.Command("whoami").Output()
			if err != nil {
				return err
			}

			whoami := strings.TrimSpace(string(whoamiResp))
			clientName = fmt.Sprintf("%s-%s", clientName, whoami)

			err = infra.CreateOvpnFile(clientName)
			if err != nil {
				return err
			}

			if v.VpnAutoConnect {
				err = infra.CreateOvpnConnection(clientName)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (v *V1alpha2) Kubernetes(dryRun bool) error {
	return nil
}

func (v *V1alpha2) Distribution(dryRun bool) error {
	return nil
}

func (v *V1alpha2) CreateTfVars(furyFile schemas.EksclusterKfdV1Alpha2Json, infraPath string) error {
	var buffer bytes.Buffer

	buffer.WriteString(fmt.Sprintf("name = \"%v\"\n", furyFile.Metadata.Name))
	buffer.WriteString(fmt.Sprintf(
		"network_cidr = \"%v\"\n",
		furyFile.Spec.Infrastructure.Vpc.Network.Cidr,
	))

	publicSubnetworkCidrs := make([]string, len(furyFile.Spec.Infrastructure.Vpc.Network.SubnetsCidrs.Public))

	for i, cidr := range furyFile.Spec.Infrastructure.Vpc.Network.SubnetsCidrs.Public {
		publicSubnetworkCidrs[i] = fmt.Sprintf("\"%v\"", cidr)
	}

	privateSubnetworkCidrs := make([]string, len(furyFile.Spec.Infrastructure.Vpc.Network.SubnetsCidrs.Private))

	for i, cidr := range furyFile.Spec.Infrastructure.Vpc.Network.SubnetsCidrs.Private {
		privateSubnetworkCidrs[i] = fmt.Sprintf("\"%v\"", cidr)
	}

	buffer.WriteString(fmt.Sprintf(
		"public_subnetwork_cidrs = [%v]\n",
		strings.Join(publicSubnetworkCidrs, ",")))

	buffer.WriteString(fmt.Sprintf(
		"private_subnetwork_cidrs = [%v]\n",
		strings.Join(privateSubnetworkCidrs, ",")))

	if furyFile.Spec.Infrastructure.Vpc.Vpn != nil {
		buffer.WriteString(
			fmt.Sprintf(
				"vpn_subnetwork_cidr = \"%v\"\n",
				furyFile.Spec.Infrastructure.Vpc.Vpn.VpnClientsSubnetCidr,
			),
		)
		buffer.WriteString(
			fmt.Sprintf(
				"vpn_instances = %v\n",
				furyFile.Spec.Infrastructure.Vpc.Vpn.Instances,
			),
		)

		if furyFile.Spec.Infrastructure.Vpc.Vpn.Port != 0 {
			buffer.WriteString(
				fmt.Sprintf(
					"vpn_port = %v\n",
					furyFile.Spec.Infrastructure.Vpc.Vpn.Port,
				),
			)
		}

		if furyFile.Spec.Infrastructure.Vpc.Vpn.InstanceType != "" {
			buffer.WriteString(
				fmt.Sprintf(
					"vpn_instance_type = \"%v\"\n",
					furyFile.Spec.Infrastructure.Vpc.Vpn.InstanceType,
				),
			)
		}

		if furyFile.Spec.Infrastructure.Vpc.Vpn.DiskSize != 0 {
			buffer.WriteString(
				fmt.Sprintf(
					"vpn_instance_disk_size = %v\n",
					furyFile.Spec.Infrastructure.Vpc.Vpn.DiskSize,
				),
			)
		}

		if furyFile.Spec.Infrastructure.Vpc.Vpn.OperatorName != "" {
			buffer.WriteString(
				fmt.Sprintf(
					"vpn_operator_name = \"%v\"\n",
					furyFile.Spec.Infrastructure.Vpc.Vpn.OperatorName,
				),
			)
		}

		if furyFile.Spec.Infrastructure.Vpc.Vpn.DhParamsBits != 0 {
			buffer.WriteString(
				fmt.Sprintf(
					"vpn_dhparams_bits = %v\n",
					furyFile.Spec.Infrastructure.Vpc.Vpn.DhParamsBits,
				),
			)
		}

		if len(furyFile.Spec.Infrastructure.Vpc.Vpn.Ssh.AllowedFromCidrs) != 0 {
			allowedCidrs := make([]string, len(furyFile.Spec.Infrastructure.Vpc.Vpn.Ssh.AllowedFromCidrs))

			for i, cidr := range furyFile.Spec.Infrastructure.Vpc.Vpn.Ssh.AllowedFromCidrs {
				allowedCidrs[i] = fmt.Sprintf("\"%v\"", cidr)
			}

			buffer.WriteString(
				fmt.Sprintf(
					"vpn_operator_cidrs = [%v]\n",
					strings.Join(allowedCidrs, ","),
				),
			)
		}

		if len(furyFile.Spec.Infrastructure.Vpc.Vpn.Ssh.GithubUsersName) != 0 {
			githubUsers := make([]string, len(furyFile.Spec.Infrastructure.Vpc.Vpn.Ssh.GithubUsersName))

			for i, gu := range furyFile.Spec.Infrastructure.Vpc.Vpn.Ssh.GithubUsersName {
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

	targetTfVars := path.Join(infraPath, "terraform", "main.auto.tfvars")

	return os.WriteFile(targetTfVars, buffer.Bytes(), 0o600)
}

func (v *V1alpha2) GetConfigFile() (schemas.EksclusterKfdV1Alpha2Json, error) {
	furyFile, err := yaml.FromFileV3[schemas.EksclusterKfdV1Alpha2Json](v.ConfigPath)
	if err != nil {
		return schemas.EksclusterKfdV1Alpha2Json{}, err
	}

	return furyFile, nil
}
