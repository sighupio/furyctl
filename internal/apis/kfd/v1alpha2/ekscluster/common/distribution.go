// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package common

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/ekscluster/v1alpha2/private"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/merge"
	"github.com/sighupio/furyctl/internal/state"
	"github.com/sighupio/furyctl/internal/template"
	"github.com/sighupio/furyctl/internal/tool/terraform"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

var (
	errCastingEbsIamToStr    = errors.New("error casting ebs_csi_driver_iam_role_arn output to string")
	errCastingLbIamToStr     = errors.New("error casting load_balancer_controller_iam_role_arn output to string")
	errCastingClsAsIamToStr  = errors.New("error casting cluster_autoscaler_iam_role_arn output to string")
	errCastingDNSPvtIamToStr = errors.New("error casting external_dns_private_iam_role_arn output to string")
	errCastingDNSPubIamToStr = errors.New("error casting external_dns_public_iam_role_arn output to string")
	errCastingCertIamToStr   = errors.New("error casting cert_manager_iam_role_arn output to string")
	errCastingVelIamToStr    = errors.New("error casting velero_iam_role_arn output to string")
)

type Distribution struct {
	*cluster.OperationPhase

	DryRun                             bool
	DistroPath                         string
	ConfigPath                         string
	InfrastructureTerraformOutputsPath string
	FuryctlConf                        private.EksclusterKfdV1Alpha2
	StateStore                         state.Storer
	TFRunner                           *terraform.Runner
}

type InjectType struct {
	Data private.SpecDistribution `json:"data"`
}

func (d *Distribution) PreparePreTerraform() (
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
		"kfd-v1alpha2",
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

	tfCfg.Data["options"] = map[any]any{
		"dryRun": d.DryRun,
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

func (d *Distribution) PreparePostTerraform(
	furyctlMerger *merge.Merger,
	preTfMerger *merge.Merger,
) (*template.Config, error) {
	postTfMerger, err := d.InjectDataPostTf(preTfMerger)
	if err != nil {
		return nil, err
	}

	mCfg, err := template.NewConfig(furyctlMerger, postTfMerger, []string{"terraform", ".gitignore"})
	if err != nil {
		return nil, fmt.Errorf("error creating template config: %w", err)
	}

	d.CopyPathsToConfig(&mCfg)

	mCfg.Data["options"] = map[any]any{
		"dryRun": d.DryRun,
	}
	mCfg.Data["checks"] = map[any]any{
		"storageClassAvailable": true,
	}

	if err = d.InjectStoredConfig(&mCfg); err != nil {
		return nil, fmt.Errorf("error injecting stored config: %w", err)
	}

	if err := d.CopyFromTemplate(
		mCfg,
		"distribution",
		path.Join(d.DistroPath, "templates", cluster.OperationPhaseDistribution),
		d.Path,
		d.ConfigPath,
	); err != nil {
		return nil, fmt.Errorf("error copying from template: %w", err)
	}

	return &mCfg, nil
}

func (d *Distribution) InjectStoredConfig(cfg *template.Config) error {
	storedCfg := map[any]any{}

	storedCfgStr, err := d.StateStore.GetConfig()
	if err != nil {
		logrus.Debugf("error while getting current config, skipping stored config injection: %s", err)

		return nil
	}

	if err = yamlx.UnmarshalV3(storedCfgStr, &storedCfg); err != nil {
		return fmt.Errorf("error while unmarshalling config file: %w", err)
	}

	cfg.Data["storedCfg"] = storedCfg

	return nil
}

func (d *Distribution) InjectDataPostTf(fMerger *merge.Merger) (*merge.Merger, error) {
	arns, err := d.extractARNsFromTfOut()
	if err != nil {
		return nil, err
	}

	injectData := InjectType{
		Data: private.SpecDistribution{
			Modules: private.SpecDistributionModules{
				Aws: &private.SpecDistributionModulesAws{
					EbsCsiDriver: private.SpecDistributionModulesAwsEbsCsiDriver{
						IamRoleArn: private.TypesAwsArn(arns["ebs_csi_driver_iam_role_arn"]),
					},
					LoadBalancerController: private.SpecDistributionModulesAwsLoadBalancerController{
						IamRoleArn: private.TypesAwsArn(arns["load_balancer_controller_iam_role_arn"]),
					},
					ClusterAutoscaler: private.SpecDistributionModulesAwsClusterAutoscaler{
						IamRoleArn: private.TypesAwsArn(arns["cluster_autoscaler_iam_role_arn"]),
					},
				},
				Ingress: private.SpecDistributionModulesIngress{
					ExternalDns: private.SpecDistributionModulesIngressExternalDNS{
						PrivateIamRoleArn: private.TypesAwsArn(arns["external_dns_private_iam_role_arn"]),
						PublicIamRoleArn:  private.TypesAwsArn(arns["external_dns_public_iam_role_arn"]),
					},
					CertManager: private.SpecDistributionModulesIngressCertManager{
						ClusterIssuer: private.SpecDistributionModulesIngressCertManagerClusterIssuer{
							Route53: private.SpecDistributionModulesIngressClusterIssuerRoute53{
								IamRoleArn: private.TypesAwsArn(arns["cert_manager_iam_role_arn"]),
							},
						},
					},
				},
			},
		},
	}

	if d.FuryctlConf.Spec.Distribution.Modules.Dr.Type == "eks" {
		injectData.Data.Modules.Dr = private.SpecDistributionModulesDr{
			Velero: &private.SpecDistributionModulesDrVelero{
				Eks: private.SpecDistributionModulesDrVeleroEks{
					IamRoleArn: private.TypesAwsArn(arns["velero_iam_role_arn"]),
				},
			},
		}
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

func (d *Distribution) extractARNsFromTfOut() (map[string]string, error) {
	arns := map[string]string{}

	out, err := d.TFRunner.Output()
	if err != nil {
		return nil, fmt.Errorf("error getting terraform output: %w", err)
	}

	ebsCsiDriverArn, ok := out["ebs_csi_driver_iam_role_arn"]
	if ok {
		arns["ebs_csi_driver_iam_role_arn"], ok = ebsCsiDriverArn.Value.(string)
		if !ok {
			return nil, errCastingEbsIamToStr
		}
	}

	loadBalancerControllerArn, ok := out["load_balancer_controller_iam_role_arn"]
	if ok {
		arns["load_balancer_controller_iam_role_arn"], ok = loadBalancerControllerArn.Value.(string)
		if !ok {
			return nil, errCastingLbIamToStr
		}
	}

	clusterAutoscalerArn, ok := out["cluster_autoscaler_iam_role_arn"]
	if ok {
		arns["cluster_autoscaler_iam_role_arn"], ok = clusterAutoscalerArn.Value.(string)
		if !ok {
			return nil, errCastingClsAsIamToStr
		}
	}

	externalDNSPrivateArn, ok := out["external_dns_private_iam_role_arn"]
	if ok {
		arns["external_dns_private_iam_role_arn"], ok = externalDNSPrivateArn.Value.(string)
		if !ok {
			return nil, errCastingDNSPvtIamToStr
		}
	}

	externalDNSPublicArn, ok := out["external_dns_public_iam_role_arn"]
	if ok {
		arns["external_dns_public_iam_role_arn"], ok = externalDNSPublicArn.Value.(string)
		if !ok {
			return nil, errCastingDNSPubIamToStr
		}
	}

	certManagerArn, ok := out["cert_manager_iam_role_arn"]
	if ok {
		arns["cert_manager_iam_role_arn"], ok = certManagerArn.Value.(string)
		if !ok {
			return nil, errCastingCertIamToStr
		}
	}

	veleroArn, ok := out["velero_iam_role_arn"]
	if ok {
		arns["velero_iam_role_arn"], ok = veleroArn.Value.(string)
		if !ok {
			return nil, errCastingVelIamToStr
		}
	}

	return arns, nil
}
