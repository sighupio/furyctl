// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eks

import (
	"encoding/json"
	"fmt"
	"github.com/sighupio/furyctl/internal/tool/kustomize"
	"github.com/sighupio/furyctl/internal/tool/terraform"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	"github.com/sirupsen/logrus"
	"os"
	"path"
	"path/filepath"

	"github.com/sighupio/fury-distribution/pkg/config"
	"github.com/sighupio/fury-distribution/pkg/schema"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/merge"
	"github.com/sighupio/furyctl/internal/template"
	mapx "github.com/sighupio/furyctl/internal/x/map"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

type Distribution struct {
	*cluster.CreationPhase
	furyctlConfPath  string
	furyctlConf      schema.EksclusterKfdV1Alpha2
	kfdManifest      config.KFD
	infraOutputsPath string
	distroPath       string
	tfRunner         *terraform.Runner
	kRunner          *kustomize.Runner
}

type injectType struct {
	Data schema.SpecDistribution `json:"data"`
}

func NewDistribution(
	furyctlConfPath string,
	furyctlConf schema.EksclusterKfdV1Alpha2,
	kfdManifest config.KFD,
	distroPath string,
	infraOutputsPath string,
) (*Distribution, error) {
	phase, err := cluster.NewCreationPhase(".distribution")
	if err != nil {
		return nil, err
	}

	return &Distribution{
		CreationPhase:    phase,
		furyctlConf:      furyctlConf,
		kfdManifest:      kfdManifest,
		infraOutputsPath: infraOutputsPath,
		distroPath:       distroPath,
		furyctlConfPath:  furyctlConfPath,
		tfRunner: terraform.NewRunner(
			execx.NewStdExecutor(),
			terraform.Paths{
				Logs:      phase.LogsPath,
				Outputs:   phase.OutputsPath,
				WorkDir:   path.Join(phase.Path, "terraform"),
				Plan:      phase.PlanPath,
				Terraform: phase.TerraformPath,
			},
		),
		kRunner: kustomize.NewRunner(
			execx.NewStdExecutor(),
			kustomize.Paths{
				Kustomize: phase.KustomizePath,
				WorkDir:   path.Join(phase.Path, "manifests"),
			},
		),
	}, nil
}

func (d *Distribution) Exec(dryRun bool) error {
	//timestamp := time.Now().Unix()

	if err := d.CreateFolder(); err != nil {
		return err
	}

	furyctlMerger, err := d.createFuryctlMerger()
	if err != nil {
		return err
	}

	injectMerger, err := d.injectDataPreTf(furyctlMerger)
	if err != nil {
		return err
	}

	tfCfg, err := d.createConfig(furyctlMerger, injectMerger, []string{"source/manifests", ".gitignore"})
	if err != nil {
		return err
	}

	if err := d.copyFromTemplate(tfCfg, dryRun); err != nil {
		return err
	}

	if err := d.CreateFolderStructure(); err != nil {
		return err
	}

	//if err := d.tfRunner.Init(); err != nil {
	//	return err
	//}
	//
	//if err := d.tfRunner.Plan(timestamp); err != nil {
	//	return err
	//}
	//
	//if dryRun {
	//	return nil
	//}

	//_, err = d.tfRunner.Apply(timestamp)
	//if err != nil {
	//	return err
	//}

	postTfMerger, err := d.injectDataPostTf(injectMerger)
	if err != nil {
		return err
	}

	mCfg, err := d.createConfig(furyctlMerger, postTfMerger, []string{"source/terraform", ".gitignore"})

	if err := d.copyFromTemplate(mCfg, dryRun); err != nil {
		return err
	}

	for m := range mapx.FromStruct(d.kfdManifest.Modules, "yaml", true) {
		modName, ok := m.(string)
		if !ok {
			return fmt.Errorf("module name: \"%v\" is not a string", m)
		}

		logrus.Debugf("Module: %s", modName)

		_, err := os.Stat(filepath.Join(d.Path, "manifests", modName, "kustomization.yaml"))
		if err != nil {
			logrus.Warnf("module %s does not have a kustomization.yaml file", modName)
			continue
		}

		kOut, err := d.kRunner.Build(modName)
		if err != nil {
			return err
		}

		os.WriteFile(path.Join(d.Path, fmt.Sprintf("kustomize-%s.yaml", modName)), []byte(kOut), 0644)
	}

	return nil
}

func (d *Distribution) createFuryctlMerger() (*merge.Merger, error) {
	defaultsFilePath := path.Join(d.distroPath, "furyctl-defaults.yaml")

	defaultsFile, err := yamlx.FromFileV2[map[any]any](defaultsFilePath)
	if err != nil {
		return &merge.Merger{}, fmt.Errorf("%s - %w", defaultsFilePath, err)
	}

	furyctlConf, err := yamlx.FromFileV2[map[any]any](d.furyctlConfPath)
	if err != nil {
		return &merge.Merger{}, fmt.Errorf("%s - %w", d.furyctlConfPath, err)
	}

	furyctlConfMergeModel := merge.NewDefaultModel(furyctlConf, ".spec.distribution")

	merger := merge.NewMerger(
		merge.NewDefaultModel(defaultsFile, ".data"),
		furyctlConfMergeModel,
	)

	_, err = merger.Merge()
	if err != nil {
		return nil, err
	}

	reverseMerger := merge.NewMerger(
		*merger.GetCustom(),
		*merger.GetBase(),
	)

	_, err = reverseMerger.Merge()
	if err != nil {
		return nil, err
	}

	return reverseMerger, nil
}

func (d *Distribution) injectDataPostTf(fMerger *merge.Merger) (*merge.Merger, error) {
	injectData := injectType{
		Data: schema.SpecDistribution{
			Modules: schema.SpecDistributionModules{
				Aws: &schema.SpecDistributionModulesAws{
					EbsCsiDriver: &schema.SpecDistributionModulesAwsEbsCsiDriver{
						IamRoleArn: "",
					},
					LoadBalancerController: &schema.SpecDistributionModulesAwsLoadBalancerController{
						IamRoleArn: "",
					},
					ClusterAutoscaler: &schema.SpecDistributionModulesAwsClusterAutoScaler{
						IamRoleArn: "",
					},
				},
				Ingress: schema.SpecDistributionModulesIngress{
					ExternalDns: schema.SpecDistributionModulesIngressExternalDNS{
						PrivateIamRoleArn: "",
						PublicIamRoleArn:  "",
					},
					CertManager: schema.SpecDistributionModulesIngressCERTManager{
						ClusterIssuer: schema.SpecDistributionModulesIngressClusterIssuer{
							Route53: &schema.SpecDistributionModulesIngressClusterIssuerRoute53{
								IamRoleArn: "",
							},
						},
					},
				},
				Dr: schema.SpecDistributionModulesDr{
					Velero: &schema.SpecDistributionModulesDrVelero{
						Eks: &schema.SpecDistributionModulesDrVeleroEks{
							IamRoleArn: "",
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

	_, err := merger.Merge()
	if err != nil {
		return nil, err
	}

	return merger, nil
}

func (d *Distribution) injectDataPreTf(fMerger *merge.Merger) (*merge.Merger, error) {
	vpcId, err := d.extractVpcIDFromPrevPhases(fMerger)
	if err != nil {
		return nil, err
	}

	if vpcId == "" {
		return fMerger, nil
	}

	injectData := injectType{
		Data: schema.SpecDistribution{
			Modules: schema.SpecDistributionModules{
				Ingress: schema.SpecDistributionModulesIngress{
					Dns: schema.SpecDistributionModulesIngressDNS{
						Private: schema.SpecDistributionModulesIngressDNSPrivate{
							VpcId: vpcId,
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
		return nil, err
	}

	return merger, nil
}

func (d *Distribution) extractVpcIDFromPrevPhases(fMerger *merge.Merger) (string, error) {
	vpcId := ""

	if infraOutJson, err := os.ReadFile(path.Join(d.infraOutputsPath, "output.json")); err == nil {
		var infraOut terraform.OutputJson

		if err := json.Unmarshal(infraOutJson, &infraOut); err == nil {
			if infraOut.Outputs["vpc_id"] == nil {
				return vpcId, fmt.Errorf("vpc_id not found in infra output")
			}

			vpcIdOut, ok := infraOut.Outputs["vpc_id"].Value.(string)
			if !ok {
				return vpcId, fmt.Errorf("error casting vpc_id output to string")
			}

			vpcId = vpcIdOut
		}
	} else {
		fModel := merge.NewDefaultModel((*fMerger.GetBase()).Content(), ".spec.kubernetes")

		kubeFromFuryctlConf, err := fModel.Get()
		if err != nil {
			return vpcId, err
		}

		vpcFromFuryctlConf, ok := kubeFromFuryctlConf["vpcId"].(string)
		if !ok {
			return vpcId, fmt.Errorf("vpcId is not a string")
		}

		vpcId = vpcFromFuryctlConf
	}

	return vpcId, nil
}

func (d *Distribution) copyFromTemplate(cfg template.Config, dryRun bool) error {
	outYaml, err := yamlx.MarshalV2(cfg)
	if err != nil {
		return err
	}

	outDirPath, err := os.MkdirTemp("", "furyctl-dist-")
	if err != nil {
		return err
	}

	confPath := filepath.Join(outDirPath, "config.yaml")

	logrus.Debugf("config path = %s", confPath)

	if err = os.WriteFile(confPath, outYaml, os.ModePerm); err != nil {
		return err
	}

	templateModel, err := template.NewTemplateModel(
		path.Join(d.distroPath, "source"),
		path.Join(d.Path),
		confPath,
		outDirPath,
		".tpl",
		false,
		dryRun,
	)
	if err != nil {
		return err
	}

	return templateModel.Generate()
}

func (d *Distribution) createConfig(fMerger, injMerger *merge.Merger, excluded []string) (template.Config, error) {
	var cfg template.Config

	mergedTmpl, ok := (*fMerger.GetCustom()).Content()["templates"]
	if !ok {
		return template.Config{}, fmt.Errorf("templates not found in merged distribution")
	}

	tmpl, err := template.NewTemplatesFromMap(mergedTmpl)
	if err != nil {
		return template.Config{}, err
	}

	tmpl.Excludes = excluded

	cfg.Templates = *tmpl
	cfg.Data = mapx.ToMapStringAny((*injMerger.GetBase()).Content())
	cfg.Include = nil

	return cfg, nil
}
