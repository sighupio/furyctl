// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package common

import (
	"encoding/json"
	"fmt"
	"os"
	"path"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/fury-distribution/pkg/apis/ekscluster/v1alpha2/private"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/merge"
	"github.com/sighupio/furyctl/internal/tool/terraform"
	"github.com/sighupio/furyctl/pkg/template"
	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
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

	return nil
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

	cfg, err = template.NewConfig(reverseMerger, reverseMerger, []string{".gitignore"})
	if err != nil {
		return cfg, fmt.Errorf("error creating template config: %w", err)
	}

	return cfg, nil
}

func (k *Kubernetes) copyFromTemplate(furyctlCfg template.Config) error {
	eksInstallerPath := path.Join(k.Path, "..", "vendor", "installers", "eks", "modules", "eks")

	nodeSelector, tolerations, err := k.getCommonDataFromDistribution(furyctlCfg)
	if err != nil {
		return err
	}

	infraData, err := k.injectInfraDataFromOutputs()
	if err != nil {
		return fmt.Errorf("error injecting infrastructure data from outputs: %w", err)
	}

	furyctlCfg.Data["infrastructure"] = infraData

	furyctlCfg.Data["kubernetes"] = map[any]any{
		"installerPath": eksInstallerPath,
		"kubectlPath":   k.KubectlPath,
		"version":       k.KFDManifest.Kubernetes.Eks.Version,
	}

	furyctlCfg.Data["distribution"] = map[any]any{
		"nodeSelector": nodeSelector,
		"tolerations":  tolerations,
	}

	furyctlCfg.Data["features"] = map[any]any{
		"logTypesEnabled": distribution.HasFeature(k.KFDManifest, distribution.FeatureKubernetesLogTypes),
	}

	err = k.CopyFromTemplate(
		furyctlCfg,
		"kubernetes",
		path.Join(
			k.DistroPath,
			"templates",
			cluster.OperationPhaseKubernetes,
			"ekscluster",
			"terraform",
		),
		path.Join(k.Path, "terraform"),
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

func (k *Kubernetes) injectInfraDataFromOutputs() (map[any]any, error) {
	out := make(map[any]any)

	if k.FuryctlConf.Spec.Infrastructure != nil &&
		k.FuryctlConf.Spec.Infrastructure.Vpc != nil {
		var infraOut terraform.OutputJSON

		infraOutJSON, err := os.ReadFile(path.Join(k.InfrastructureTerraformOutputsPath, "output.json"))
		if err != nil {
			return out, fmt.Errorf("error reading infrastructure terraform outputs: %w", err)
		}

		if err := json.Unmarshal(infraOutJSON, &infraOut); err != nil {
			return out, fmt.Errorf("error unmarshalling infrastructure terraform outputs: %w", err)
		}

		if infraOut["private_subnets"] == nil {
			return out, ErrPvtSubnetNotFound
		}

		s, ok := infraOut["private_subnets"].Value.([]any)
		if !ok {
			return out, ErrPvtSubnetFromOut
		}

		if infraOut["vpc_id"] == nil {
			return out, ErrVpcIDNotFound
		}

		v, ok := infraOut["vpc_id"].Value.(string)
		if !ok {
			return out, ErrVpcIDFromOut
		}

		if infraOut["vpc_cidr_block"] == nil {
			return out, ErrVpcCIDRNotFound
		}

		c, ok := infraOut["vpc_cidr_block"].Value.(string)
		if !ok {
			return out, ErrVpcCIDRFromOut
		}

		subs := make([]private.TypesAwsSubnetId, len(s))

		for i, sub := range s {
			ss, ok := sub.(string)
			if !ok {
				return out, ErrPvtSubnetFromOut
			}

			subs[i] = private.TypesAwsSubnetId(ss)
		}

		vpcID := private.TypesAwsVpcId(v)

		out["subnets"] = subs

		out["vpcId"] = &vpcID

		out["clusterEndpointPrivateAccessCidrs"] = private.TypesCidr(c)
	}

	return out, nil
}
