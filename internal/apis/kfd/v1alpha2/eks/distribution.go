// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eks

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/sighupio/fury-distribution/pkg/config"
	"github.com/sighupio/fury-distribution/pkg/schema"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/merge"
	"github.com/sighupio/furyctl/internal/template"
	"github.com/sighupio/furyctl/internal/tool/kubectl"
	"github.com/sighupio/furyctl/internal/tool/kustomize"
	"github.com/sighupio/furyctl/internal/tool/terraform"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
	"github.com/sirupsen/logrus"
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
	kubeRunner       *kubectl.Runner
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
		kubeRunner: kubectl.NewRunner(
			execx.NewStdExecutor(),
			kubectl.Paths{
				Kubectl: phase.KubectlPath,
				WorkDir: path.Join(phase.Path, "manifests"),
			},
		),
	}, nil
}

func (d *Distribution) Exec(dryRun bool) error {
	timestamp := time.Now().Unix()

	if err := d.CreateFolder(); err != nil {
		return err
	}

	furyctlMerger, err := d.createFuryctlMerger()
	if err != nil {
		return err
	}

	preTfMerger, err := d.injectDataPreTf(furyctlMerger)
	if err != nil {
		return err
	}

	tfCfg, err := template.NewConfig(furyctlMerger, preTfMerger, []string{"source/manifests", ".gitignore"})
	if err != nil {
		return err
	}

	if err := d.copyFromTemplate(tfCfg, dryRun); err != nil {
		return err
	}

	if err := d.CreateFolderStructure(); err != nil {
		return err
	}

	if err := d.tfRunner.Init(); err != nil {
		return err
	}

	if err := d.tfRunner.Plan(timestamp); err != nil {
		return err
	}

	if dryRun {
		return nil
	}

	_, err = d.tfRunner.Apply(timestamp)
	if err != nil {
		return err
	}

	postTfMerger, err := d.injectDataPostTf(preTfMerger)
	if err != nil {
		return err
	}

	mCfg, err := template.NewConfig(furyctlMerger, postTfMerger, []string{"source/terraform", ".gitignore"})

	if err := d.copyFromTemplate(mCfg, dryRun); err != nil {
		return err
	}

	logrus.Info("Building manifests")

	manifestsOutPath, err := d.buildManifests()
	if err != nil {
		return err
	}

	logrus.Info("Applying manifests")

	return d.applyManifests(manifestsOutPath)
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

func (d *Distribution) injectDataPostTf(fMerger *merge.Merger) (*merge.Merger, error) {
	arns, err := d.extractARNsFromTfOut()
	if err != nil {
		return nil, err
	}

	injectData := injectType{
		Data: schema.SpecDistribution{
			Modules: schema.SpecDistributionModules{
				Aws: &schema.SpecDistributionModulesAws{
					EbsCsiDriver: &schema.SpecDistributionModulesAwsEbsCsiDriver{
						IamRoleArn: schema.TypesAwsArn(arns["ebs_csi_driver_iam_role_arn"]),
					},
					LoadBalancerController: &schema.SpecDistributionModulesAwsLoadBalancerController{
						IamRoleArn: schema.TypesAwsArn(arns["load_balancer_controller_iam_role_arn"]),
					},
					ClusterAutoscaler: &schema.SpecDistributionModulesAwsClusterAutoScaler{
						IamRoleArn: schema.TypesAwsArn(arns["cluster_autoscaler_iam_role_arn"]),
					},
				},
				Ingress: schema.SpecDistributionModulesIngress{
					ExternalDns: schema.SpecDistributionModulesIngressExternalDNS{
						PrivateIamRoleArn: schema.TypesAwsArn(arns["external_dns_private_iam_role_arn"]),
						PublicIamRoleArn:  schema.TypesAwsArn(arns["external_dns_public_iam_role_arn"]),
					},
					CertManager: schema.SpecDistributionModulesIngressCERTManager{
						ClusterIssuer: schema.SpecDistributionModulesIngressClusterIssuer{
							Route53: &schema.SpecDistributionModulesIngressClusterIssuerRoute53{
								IamRoleArn: schema.TypesAwsArn(arns["cert_manager_iam_role_arn"]),
							},
						},
					},
				},
				Dr: schema.SpecDistributionModulesDr{
					Velero: &schema.SpecDistributionModulesDrVelero{
						Eks: &schema.SpecDistributionModulesDrVeleroEks{
							IamRoleArn: schema.TypesAwsArn(arns["velero_iam_role_arn"]),
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

func (d *Distribution) extractARNsFromTfOut() (map[string]string, error) {
	var distroOut terraform.OutputJson

	arns := map[string]string{}

	distroOutJson, err := os.ReadFile(path.Join(d.OutputsPath, "output.json"))
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(distroOutJson, &distroOut); err != nil {
		return nil, err
	}

	ebsCsiDriverArn, ok := distroOut.Outputs["ebs_csi_driver_iam_role_arn"]
	if ok {
		arns["ebs_csi_driver_iam_role_arn"] = ebsCsiDriverArn.Value.(string)
	}

	loadBalancerControllerArn, ok := distroOut.Outputs["load_balancer_controller_iam_role_arn"]
	if ok {
		arns["load_balancer_controller_iam_role_arn"] = loadBalancerControllerArn.Value.(string)
	}

	clusterAutoscalerArn, ok := distroOut.Outputs["cluster_autoscaler_iam_role_arn"]
	if ok {
		arns["cluster_autoscaler_iam_role_arn"] = clusterAutoscalerArn.Value.(string)
	}

	externalDnsPrivateArn, ok := distroOut.Outputs["external_dns_private_iam_role_arn"]
	if ok {
		arns["external_dns_private_iam_role_arn"] = externalDnsPrivateArn.Value.(string)
	}

	externalDnsPublicArn, ok := distroOut.Outputs["external_dns_public_iam_role_arn"]
	if ok {
		arns["external_dns_public_iam_role_arn"] = externalDnsPublicArn.Value.(string)
	}

	certManagerArn, ok := distroOut.Outputs["cert_manager_iam_role_arn"]
	if ok {
		arns["cert_manager_iam_role_arn"] = certManagerArn.Value.(string)
	}

	veleroArn, ok := distroOut.Outputs["velero_iam_role_arn"]
	if ok {
		arns["velero_iam_role_arn"] = veleroArn.Value.(string)
	}

	return arns, nil
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

func (d *Distribution) buildManifests() (string, error) {
	kOut, err := d.kRunner.Build()
	if err != nil {
		return "", err
	}

	outDirPath, err := os.MkdirTemp("", "furyctl-dist-manifests-")
	if err != nil {
		return "", err
	}

	manifestsOutPath := filepath.Join(outDirPath, "out.yaml")

	logrus.Debugf("built manifests = %s", manifestsOutPath)

	if err = os.WriteFile(manifestsOutPath, []byte(kOut), os.ModePerm); err != nil {
		return "", err
	}

	return manifestsOutPath, nil
}

func (d *Distribution) applyManifests(path string) error {
	var err error

	maxRetry := 3

	for i := 0; i < maxRetry; i++ {
		err = d.kubeRunner.Apply(path, true)
	}

	return err
}
