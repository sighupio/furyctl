// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package create

import (
	"bytes"
	"errors"
	"fmt"
	schm "github.com/sighupio/fury-distribution/pkg/schemas"
	"github.com/sighupio/furyctl/cmd/validate"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/template"
	"github.com/sighupio/furyctl/internal/yaml"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"os"
	"path"
	"strings"
)

var MissingDependenciesError = fmt.Errorf("missing dependencies")

type EksInfraTFConf struct {
	Data struct {
		Eks struct {
			version string `yaml:"version"`
		} `yaml:"eks"`
	} `yaml:"data"`
}

func NewClusterCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "Creates a battle-tested Kubernetes cluster",
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			//debug := validate.Flag[bool](cmd, "debug").(bool)
			furyctlPath := validate.Flag[string](cmd, "config").(string)
			distroLocation := validate.Flag[string](cmd, "distro-location").(string)
			phase := validate.Flag[string](cmd, "phase").(string)
			vendorPath := path.Join("vendor")

			minimalConf, err := yaml.FromFileV3[distribution.FuryctlConfig](furyctlPath)
			if err != nil {
				return err
			}

			err = ValidateConfig(furyctlPath, distroLocation)
			if err != nil {
				return err
			}

			err = ValidateEnv(furyctlPath)
			if err != nil {
				return err
			}

			err = ValidateDeps(furyctlPath, distroLocation, vendorPath)
			if err != nil {
				if errors.Is(err, MissingDependenciesError) {
					err := DownloadDeps(furyctlPath, distroLocation, vendorPath)
					if err != nil {
						return err
					}
				} else {
					return err
				}
			}

			err = DownloadRequirements(furyctlPath, distroLocation, vendorPath)
			if err != nil {
				return err
			}

			if phase != "" {
				logrus.Infof("Running phase: %s", phase)

				switch phase {
				case "infrastructure":
					err = InfrastructurePhase(furyctlPath, distroLocation, vendorPath)
				case "kubernetes":
					err = KubernetesPhase(furyctlPath, distroLocation, vendorPath)
				case "distribution":
					err = DistributionPhase(furyctlPath, distroLocation, vendorPath)
				default:
					return fmt.Errorf("unknown phase: %s", phase)
				}
				return err
			}

			if minimalConf.Spec.Infrastructure != nil {
				logrus.Infoln("Running infrastructure phase")

				err = InfrastructurePhase(furyctlPath, distroLocation, vendorPath)
				if err != nil {
					return err
				}
			}

			logrus.Infoln("Running kubernetes phase")
			err = KubernetesPhase(furyctlPath, distroLocation, vendorPath)
			if err != nil {
				return err
			}

			logrus.Infoln("Running distribution phase")
			err = DistributionPhase(furyctlPath, distroLocation, vendorPath)
			if err != nil {
				return err
			}

			return nil
		},
	}

	cmd.Flags().StringP(
		"config",
		"c",
		"furyctl.yaml",
		"Path to the furyctl.yaml file",
	)

	cmd.Flags().StringP(
		"phase",
		"p",
		"",
		"Phase to execute",
	)

	cmd.Flags().StringP(
		"distro-location",
		"",
		"",
		"Base URL used to download schemas, defaults and the distribution manifest. "+
			"It can either be a local path(eg: /path/to/fury/distribution) or "+
			"a remote URL(eg: https://git@github.com/sighupio/fury-distribution?ref=BRANCH_NAME)."+
			"Any format supported by hashicorp/go-getter can be used.",
	)

	cmd.Flags().Bool(
		"dry-run",
		false,
		"Allows to inspect what resources will be created before applying them",
	)

	return cmd
}

func ValidateConfig(configPath, distroLocation string) error {
	return nil
}

func ValidateDeps(configPath, distroLocation, binPath string) error {
	return nil
}

func ValidateEnv(configPath string) error {
	return nil
}

func DownloadDeps(configPath, distroLocation, binPath string) error {
	return nil
}

func DownloadRequirements(configPath, distroLocation, dlPath string) error {
	return nil
}

func InfrastructurePhase(configPath, distroLocation, dlPath string) error {
	var config template.Config

	err := os.Mkdir(".infrastructure", 0o755)
	if err != nil {
		return err
	}

	sourceTfDir := path.Join("", "data", "provisioners", "bootstrap", "aws")
	targetTfDir := path.Join(".infrastructure", "terraform")

	outDirPath, err := os.MkdirTemp("", "furyctl-infra-")
	if err != nil {
		return err
	}

	repoPath, err := validate.DownloadDirectory(distroLocation)
	if err != nil {
		return err
	}

	minimalConf, err := yaml.FromFileV3[distribution.FuryctlConfig](configPath)
	if err != nil {
		return err
	}

	kfdPath := path.Join(repoPath, "kfd.yaml")
	kfdManifest, err := yaml.FromFileV2[distribution.Manifest](kfdPath)
	if err != nil {
		return err
	}

	tfConfVars := map[string]map[any]any{
		"kubernetes": {
			"eks": kfdManifest.Kubernetes.Eks,
		},
	}

	config.Data = tfConfVars

	tfConfigPath := path.Join(outDirPath, "tf-config.yaml")
	tfConfig, err := yaml.MarshalV2(config)
	if err != nil {
		return err
	}

	err = os.WriteFile(tfConfigPath, tfConfig, 0o644)
	if err != nil {
		return err
	}

	templateModel, err := template.NewTemplateModel(
		sourceTfDir,
		targetTfDir,
		tfConfigPath,
		outDirPath,
		".tpl",
		true,
		false,
	)
	if err != nil {
		return err
	}

	err = templateModel.Generate()

	switch minimalConf.ApiVersion {
	case "kfd.sighup.io/v1alpha2":
		logrus.Infoln("Generating tvars file")
		var buffer bytes.Buffer

		furyFile, err := yaml.FromFileV3[schm.EksclusterKfdV1Alpha2Json](configPath)
		if err != nil {
			return err
		}

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
			buffer.WriteString(fmt.Sprintf("vpn_subnetwork_cidr = \"%v\"\n", furyFile.Spec.Infrastructure.Vpc.Vpn.VpnClientsSubnetCidr))
			buffer.WriteString(fmt.Sprintf("vpn_instances = %v\n", furyFile.Spec.Infrastructure.Vpc.Vpn.Instances))

			if furyFile.Spec.Infrastructure.Vpc.Vpn.Port != 0 {
				buffer.WriteString(fmt.Sprintf("vpn_port = %v\n", furyFile.Spec.Infrastructure.Vpc.Vpn.Port))
			}

			if furyFile.Spec.Infrastructure.Vpc.Vpn.InstanceType != "" {
				buffer.WriteString(fmt.Sprintf("vpn_instance_type = \"%v\"\n", furyFile.Spec.Infrastructure.Vpc.Vpn.InstanceType))
			}

			if furyFile.Spec.Infrastructure.Vpc.Vpn.DiskSize != 0 {
				buffer.WriteString(fmt.Sprintf("vpn_instance_disk_size = %v\n", furyFile.Spec.Infrastructure.Vpc.Vpn.DiskSize))
			}

			if furyFile.Spec.Infrastructure.Vpc.Vpn.OperatorName != "" {
				buffer.WriteString(fmt.Sprintf("vpn_operator_name = \"%v\"\n", furyFile.Spec.Infrastructure.Vpc.Vpn.OperatorName))
			}

			if furyFile.Spec.Infrastructure.Vpc.Vpn.DhParamsBits != 0 {
				buffer.WriteString(fmt.Sprintf("vpn_dhparams_bits = %v\n", furyFile.Spec.Infrastructure.Vpc.Vpn.DhParamsBits))
			}

			if len(furyFile.Spec.Infrastructure.Vpc.Vpn.Ssh.AllowedFromCidrs) != 0 {
				allowedCidrs := make([]string, len(furyFile.Spec.Infrastructure.Vpc.Vpn.Ssh.AllowedFromCidrs))

				for i, cidr := range furyFile.Spec.Infrastructure.Vpc.Vpn.Ssh.AllowedFromCidrs {
					allowedCidrs[i] = fmt.Sprintf("\"%v\"", cidr)
				}

				buffer.WriteString(fmt.Sprintf("vpn_operator_cidrs = [%v]\n", strings.Join(allowedCidrs, ",")))
			}

			if len(furyFile.Spec.Infrastructure.Vpc.Vpn.Ssh.GithubUsersName) != 0 {
				githubUsers := make([]string, len(furyFile.Spec.Infrastructure.Vpc.Vpn.Ssh.GithubUsersName))

				for i, gu := range furyFile.Spec.Infrastructure.Vpc.Vpn.Ssh.GithubUsersName {
					githubUsers[i] = fmt.Sprintf("\"%v\"", gu)
				}

				buffer.WriteString(fmt.Sprintf("vpn_ssh_users = [%v]\n", strings.Join(githubUsers, ",")))
			}
		}

		targetTfVars := path.Join(".infrastructure", "terraform", "aws.tfvars")

		err = os.WriteFile(targetTfVars, buffer.Bytes(), 0600)
		if err != nil {
			return err
		}

	default:
		return fmt.Errorf("unknown apiVersion: %s", minimalConf.ApiVersion)
	}

	return err
}

func KubernetesPhase(configPath, distroLocation, dlPath string) error {
	err := os.Mkdir(".kubernetes", 0o755)
	if err != nil {
		return err
	}

	return nil
}

func DistributionPhase(configPath, distroLocation, dlPath string) error {
	err := os.Mkdir(".distribution", 0o755)
	if err != nil {
		return err
	}

	return nil
}
