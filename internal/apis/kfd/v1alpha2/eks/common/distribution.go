// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package common

import (
	"encoding/json"
	"fmt"
	"os"
	"path"

	"github.com/sighupio/fury-distribution/pkg/apis/ekscluster/v1alpha2/private"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/merge"
	"github.com/sighupio/furyctl/internal/template"
	"github.com/sighupio/furyctl/internal/tool/terraform"
)

type Distribution struct {
	*cluster.OperationPhase

	DryRun                             bool
	DistroPath                         string
	ConfigPath                         string
	InfrastructureTerraformOutputsPath string
}

type InjectType struct {
	Data private.SpecDistribution `json:"data"`
}

func (d *Distribution) Prepare() (
	*merge.Merger,
	*merge.Merger,
	*template.Config,
	error,
) {
	if err := d.CreateRootFolder(); err != nil {
		return nil, nil, nil, fmt.Errorf("error creating distribution phase folder: %w", err)
	}

	furyctlMerger, err := d.CreateFuryctlMerger(
		d.DistroPath,
		d.ConfigPath,
		"ekscluster",
	)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error creating furyctl merger: %w", err)
	}

	preTfMerger, err := d.injectDataPreTf(furyctlMerger)
	if err != nil {
		return nil, nil, nil, err
	}

	tfCfg, err := template.NewConfig(furyctlMerger, preTfMerger, []string{"manifests", "scripts", ".gitignore"})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error creating template config: %w", err)
	}

	if err := d.CopyFromTemplate(
		tfCfg,
		"distribution",
		path.Join(d.DistroPath, "templates", cluster.OperationPhaseDistribution),
		d.Path,
		d.ConfigPath,
	); err != nil {
		return nil, nil, nil, fmt.Errorf("error copying from template: %w", err)
	}

	if err := d.CreateTerraformFolderStructure(); err != nil {
		return nil, nil, nil, fmt.Errorf("error creating distribution phase folder structure: %w", err)
	}

	return furyctlMerger, preTfMerger, &tfCfg, nil
}

func (d *Distribution) injectDataPreTf(fMerger *merge.Merger) (*merge.Merger, error) {
	vpcID, err := d.extractVpcIDFromPrevPhases(fMerger)
	if err != nil {
		return nil, err
	}

	if vpcID == "" {
		return fMerger, nil
	}

	injectData := InjectType{
		Data: private.SpecDistribution{
			Modules: private.SpecDistributionModules{
				Ingress: private.SpecDistributionModulesIngress{
					Dns: private.SpecDistributionModulesIngressDNS{
						Private: private.SpecDistributionModulesIngressDNSPrivate{
							VpcId: vpcID,
						},
					},
				},
			},
		},
	}

	injectDataModel := merge.NewDefaultModelFromStruct(injectData, ".data", true)

	merger := merge.NewMerger(
		*fMerger.GetBase(),
		injectDataModel,
	)

	_, err = merger.Merge()
	if err != nil {
		return nil, fmt.Errorf("error merging furyctl config: %w", err)
	}

	return merger, nil
}

func (d *Distribution) extractVpcIDFromPrevPhases(fMerger *merge.Merger) (string, error) {
	vpcID := ""

	if infraOutJSON, err := os.ReadFile(path.Join(d.InfrastructureTerraformOutputsPath, "output.json")); err == nil {
		var infraOut terraform.OutputJSON

		if err := json.Unmarshal(infraOutJSON, &infraOut); err == nil {
			if infraOut["vpc_id"] == nil {
				return vpcID, ErrVpcIDNotFound
			}

			vpcIDOut, ok := infraOut["vpc_id"].Value.(string)
			if !ok {
				return vpcID, ErrCastingVpcIDToStr
			}

			vpcID = vpcIDOut
		}
	} else {
		fModel := merge.NewDefaultModel((*fMerger.GetBase()).Content(), ".spec.kubernetes")

		kubeFromFuryctlConf, err := fModel.Get()
		if err != nil {
			return vpcID, fmt.Errorf("error getting kubernetes from furyctl config: %w", err)
		}

		vpcFromFuryctlConf, ok := kubeFromFuryctlConf["vpcId"].(string)
		if !ok && !d.DryRun {
			return vpcID, ErrCastingVpcIDToStr
		}

		vpcID = vpcFromFuryctlConf
	}

	return vpcID, nil
}
