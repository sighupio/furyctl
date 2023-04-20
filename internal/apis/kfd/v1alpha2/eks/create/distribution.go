// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package create

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/config"
	"github.com/sighupio/fury-distribution/pkg/schema/private"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/merge"
	"github.com/sighupio/furyctl/internal/template"
	"github.com/sighupio/furyctl/internal/tool/kubectl"
	"github.com/sighupio/furyctl/internal/tool/kustomize"
	"github.com/sighupio/furyctl/internal/tool/terraform"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

const (
	kubectlDelayMaxRetry   = 3
	kubectlNoDelayMaxRetry = 7
)

var (
	errCastingVpcIDToStr     = errors.New("error casting vpc_id output to string")
	errCastingEbsIamToStr    = errors.New("error casting ebs_csi_driver_iam_role_arn output to string")
	errCastingLbIamToStr     = errors.New("error casting load_balancer_controller_iam_role_arn output to string")
	errCastingClsAsIamToStr  = errors.New("error casting cluster_autoscaler_iam_role_arn output to string")
	errCastingDNSPvtIamToStr = errors.New("error casting external_dns_private_iam_role_arn output to string")
	errCastingDNSPubIamToStr = errors.New("error casting external_dns_public_iam_role_arn output to string")
	errCastingCertIamToStr   = errors.New("error casting cert_manager_iam_role_arn output to string")
	errCastingVelIamToStr    = errors.New("error casting velero_iam_role_arn output to string")
	errClusterConnect        = errors.New("error connecting to cluster")
)

type Distribution struct {
	*cluster.OperationPhase
	furyctlConfPath  string
	furyctlConf      private.EksclusterKfdV1Alpha2
	kfdManifest      config.KFD
	infraOutputsPath string
	distroPath       string
	tfRunner         *terraform.Runner
	kzRunner         *kustomize.Runner
	kubeRunner       *kubectl.Runner
	dryRun           bool
	phase            string
}

type injectType struct {
	Data private.SpecDistribution `json:"data"`
}

func NewDistribution(
	paths cluster.CreatorPaths,
	furyctlConf private.EksclusterKfdV1Alpha2,
	kfdManifest config.KFD,
	infraOutputsPath string,
	dryRun bool,
	phase string,
) (*Distribution, error) {
	distroDir := path.Join(paths.WorkDir, cluster.OperationPhaseDistribution)

	phaseOp, err := cluster.NewOperationPhase(distroDir, kfdManifest.Tools, paths.BinPath)
	if err != nil {
		return nil, fmt.Errorf("error creating distribution phase: %w", err)
	}

	return &Distribution{
		OperationPhase:   phaseOp,
		furyctlConf:      furyctlConf,
		kfdManifest:      kfdManifest,
		infraOutputsPath: infraOutputsPath,
		distroPath:       paths.DistroPath,
		furyctlConfPath:  paths.ConfigPath,
		tfRunner: terraform.NewRunner(
			execx.NewStdExecutor(),
			terraform.Paths{
				Logs:      phaseOp.LogsPath,
				Outputs:   phaseOp.OutputsPath,
				WorkDir:   path.Join(phaseOp.Path, "terraform"),
				Plan:      phaseOp.PlanPath,
				Terraform: phaseOp.TerraformPath,
			},
		),
		kzRunner: kustomize.NewRunner(
			execx.NewStdExecutor(),
			kustomize.Paths{
				Kustomize: phaseOp.KustomizePath,
				WorkDir:   path.Join(phaseOp.Path, "manifests"),
			},
		),
		kubeRunner: kubectl.NewRunner(
			execx.NewStdExecutor(),
			kubectl.Paths{
				Kubectl:    phaseOp.KubectlPath,
				WorkDir:    path.Join(phaseOp.Path, "manifests"),
				Kubeconfig: paths.Kubeconfig,
			},
			true,
			true,
			false,
		),
		dryRun: dryRun,
		phase:  phase,
	}, nil
}

func (d *Distribution) Exec() error {
	timestamp := time.Now().Unix()

	logrus.Info("Installing Kubernetes Fury Distribution...")

	logrus.Debug("Create: running distribution phase...")

	logrus.Info("Checking that the cluster is reachable...")

	if _, err := d.kubeRunner.Version(); err != nil {
		logrus.Debugf("Got error while running cluster reachability check: %s", err)

		if !d.dryRun {
			return errClusterConnect
		}

		if d.phase == cluster.OperationPhaseDistribution {
			logrus.Warnf("Cluster is unreachable, make sure it is reachable before " +
				"running the command without --dry-run")
		}
	}

	if err := d.CreateFolder(); err != nil {
		return fmt.Errorf("error creating distribution phase folder: %w", err)
	}

	furyctlMerger, err := d.createFuryctlMerger()
	if err != nil {
		return err
	}

	preTfMerger, err := d.injectDataPreTf(furyctlMerger)
	if err != nil {
		return err
	}

	tfCfg, err := template.NewConfig(furyctlMerger, preTfMerger, []string{"manifests", ".gitignore"})
	if err != nil {
		return fmt.Errorf("error creating template config: %w", err)
	}

	if err := d.copyFromTemplate(tfCfg); err != nil {
		return err
	}

	if err := d.CreateFolderStructure(); err != nil {
		return fmt.Errorf("error creating distribution phase folder structure: %w", err)
	}

	if err := d.tfRunner.Init(); err != nil {
		return fmt.Errorf("error running terraform init: %w", err)
	}

	if err := d.tfRunner.Plan(timestamp); err != nil && !d.dryRun {
		return fmt.Errorf("error running terraform plan: %w", err)
	}

	if d.dryRun {
		if err := d.createDummyOutput(); err != nil {
			return fmt.Errorf("error creating dummy output: %w", err)
		}

		postTfMerger, err := d.injectDataPostTf(preTfMerger)
		if err != nil {
			return err
		}

		mCfg, err := template.NewConfig(furyctlMerger, postTfMerger, []string{"terraform", ".gitignore"})
		if err != nil {
			return fmt.Errorf("error creating template config: %w", err)
		}

		if err := d.copyFromTemplate(mCfg); err != nil {
			return err
		}

		_, err = d.buildManifests()
		if err != nil {
			return err
		}

		return nil
	}

	logrus.Warn("Creating cloud resources, this could take a while...")

	_, err = d.tfRunner.Apply(timestamp)
	if err != nil {
		return fmt.Errorf("cannot create cloud resources: %w", err)
	}

	postTfMerger, err := d.injectDataPostTf(preTfMerger)
	if err != nil {
		return err
	}

	mCfg, err := template.NewConfig(furyctlMerger, postTfMerger, []string{"terraform", ".gitignore"})
	if err != nil {
		return fmt.Errorf("error creating template config: %w", err)
	}

	if err := d.copyFromTemplate(mCfg); err != nil {
		return err
	}

	logrus.Info("Building manifests...")

	manifestsOutPath, err := d.buildManifests()
	if err != nil {
		return err
	}

	logrus.Info("Applying manifests...")

	return d.applyManifests(manifestsOutPath)
}

func (d *Distribution) Stop() []error {
	// use gorooutines to stop all the tools
	var wg sync.WaitGroup
	errChan := make(chan error, 1)

	wg.Add(3)

	go func() {
		defer wg.Done()
		if err := d.tfRunner.Stop(); err != nil {
			errChan <- err
		}
	}()

	go func() {
		defer wg.Done()
		if err := d.kzRunner.Stop(); err != nil {
			errChan <- err
		}
	}()

	go func() {
		defer wg.Done()
		if err := d.kubeRunner.Stop(); err != nil {
			errChan <- err
		}
	}()

	wg.Wait()

	close(errChan)

	errs := make([]error, 0)

	for err := range errChan {
		errs = append(errs, err)
	}

	return errs
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
		return nil, fmt.Errorf("error merging furyctl config: %w", err)
	}

	reverseMerger := merge.NewMerger(
		*merger.GetCustom(),
		*merger.GetBase(),
	)

	_, err = reverseMerger.Merge()
	if err != nil {
		return nil, fmt.Errorf("error merging furyctl config: %w", err)
	}

	return reverseMerger, nil
}

func (d *Distribution) injectDataPreTf(fMerger *merge.Merger) (*merge.Merger, error) {
	vpcID, err := d.extractVpcIDFromPrevPhases(fMerger)
	if err != nil {
		return nil, err
	}

	if vpcID == "" {
		return fMerger, nil
	}

	injectData := injectType{
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

	if infraOutJSON, err := os.ReadFile(path.Join(d.infraOutputsPath, "output.json")); err == nil {
		var infraOut terraform.OutputJSON

		if err := json.Unmarshal(infraOutJSON, &infraOut); err == nil {
			if infraOut.Outputs["vpc_id"] == nil {
				return vpcID, ErrVpcIDNotFound
			}

			vpcIDOut, ok := infraOut.Outputs["vpc_id"].Value.(string)
			if !ok {
				return vpcID, errCastingVpcIDToStr
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
		if !ok && !d.dryRun {
			return vpcID, errCastingVpcIDToStr
		}

		vpcID = vpcFromFuryctlConf
	}

	return vpcID, nil
}

func (d *Distribution) injectDataPostTf(fMerger *merge.Merger) (*merge.Merger, error) {
	arns, err := d.extractARNsFromTfOut()
	if err != nil {
		return nil, err
	}

	injectData := injectType{
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
				Dr: private.SpecDistributionModulesDr{
					Velero: private.SpecDistributionModulesDrVelero{
						Eks: private.SpecDistributionModulesDrVeleroEks{
							IamRoleArn: private.TypesAwsArn(arns["velero_iam_role_arn"]),
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

func (d *Distribution) createDummyOutput() error {
	arns := map[string]string{
		"ebs_csi_driver_iam_role_arn":           "arn:aws:iam::123456789012:role/dummy",
		"load_balancer_controller_iam_role_arn": "arn:aws:iam::123456789012:role/dummy",
		"cluster_autoscaler_iam_role_arn":       "arn:aws:iam::123456789012:role/dummy",
		"external_dns_private_iam_role_arn":     "arn:aws:iam::123456789012:role/dummy",
		"external_dns_public_iam_role_arn":      "arn:aws:iam::123456789012:role/dummy",
		"cert_manager_iam_role_arn":             "arn:aws:iam::123456789012:role/dummy",
		"velero_iam_role_arn":                   "arn:aws:iam::123456789012:role/dummy",
	}

	outputFilePath := path.Join(d.OutputsPath, "output.json")

	if _, err := os.Stat(outputFilePath); err == nil {
		return nil
	}

	if err := os.MkdirAll(d.OutputsPath, iox.FullPermAccess); err != nil {
		return fmt.Errorf("error while creating outputs folder: %w", err)
	}

	arnsJSON, err := json.Marshal(arns)
	if err != nil {
		return fmt.Errorf("error while marshaling arns: %w", err)
	}

	if err := os.WriteFile(outputFilePath, arnsJSON, iox.RWPermAccess); err != nil {
		return fmt.Errorf("error while creating dummy output.json: %w", err)
	}

	return nil
}

func (d *Distribution) extractARNsFromTfOut() (map[string]string, error) {
	var distroOut terraform.OutputJSON

	arns := map[string]string{}

	distroOutJSON, err := os.ReadFile(path.Join(d.OutputsPath, "output.json"))
	if err != nil {
		return nil, fmt.Errorf("error reading distribution output: %w", err)
	}

	if err := json.Unmarshal(distroOutJSON, &distroOut); err != nil {
		return nil, fmt.Errorf("error unmarshaling distribution output: %w", err)
	}

	ebsCsiDriverArn, ok := distroOut.Outputs["ebs_csi_driver_iam_role_arn"]
	if ok {
		arns["ebs_csi_driver_iam_role_arn"], ok = ebsCsiDriverArn.Value.(string)
		if !ok {
			return nil, errCastingEbsIamToStr
		}
	}

	loadBalancerControllerArn, ok := distroOut.Outputs["load_balancer_controller_iam_role_arn"]
	if ok {
		arns["load_balancer_controller_iam_role_arn"], ok = loadBalancerControllerArn.Value.(string)
		if !ok {
			return nil, errCastingLbIamToStr
		}
	}

	clusterAutoscalerArn, ok := distroOut.Outputs["cluster_autoscaler_iam_role_arn"]
	if ok {
		arns["cluster_autoscaler_iam_role_arn"], ok = clusterAutoscalerArn.Value.(string)
		if !ok {
			return nil, errCastingClsAsIamToStr
		}
	}

	externalDNSPrivateArn, ok := distroOut.Outputs["external_dns_private_iam_role_arn"]
	if ok {
		arns["external_dns_private_iam_role_arn"], ok = externalDNSPrivateArn.Value.(string)
		if !ok {
			return nil, errCastingDNSPvtIamToStr
		}
	}

	externalDNSPublicArn, ok := distroOut.Outputs["external_dns_public_iam_role_arn"]
	if ok {
		arns["external_dns_public_iam_role_arn"], ok = externalDNSPublicArn.Value.(string)
		if !ok {
			return nil, errCastingDNSPubIamToStr
		}
	}

	certManagerArn, ok := distroOut.Outputs["cert_manager_iam_role_arn"]
	if ok {
		arns["cert_manager_iam_role_arn"], ok = certManagerArn.Value.(string)
		if !ok {
			return nil, errCastingCertIamToStr
		}
	}

	veleroArn, ok := distroOut.Outputs["velero_iam_role_arn"]
	if ok {
		arns["velero_iam_role_arn"], ok = veleroArn.Value.(string)
		if !ok {
			return nil, errCastingVelIamToStr
		}
	}

	return arns, nil
}

func (d *Distribution) copyFromTemplate(cfg template.Config) error {
	outYaml, err := yamlx.MarshalV2(cfg)
	if err != nil {
		return fmt.Errorf("error marshaling template config: %w", err)
	}

	outDirPath, err := os.MkdirTemp("", "furyctl-dist-")
	if err != nil {
		return fmt.Errorf("error creating temp dir: %w", err)
	}

	confPath := filepath.Join(outDirPath, "config.yaml")

	logrus.Debugf("config path = %s", confPath)

	if err = os.WriteFile(confPath, outYaml, os.ModePerm); err != nil {
		return fmt.Errorf("error writing config file: %w", err)
	}

	templateModel, err := template.NewTemplateModel(
		path.Join(d.distroPath, "templates", cluster.OperationPhaseDistribution),
		path.Join(d.Path),
		confPath,
		outDirPath,
		".tpl",
		false,
		d.dryRun,
	)
	if err != nil {
		return fmt.Errorf("error creating template model: %w", err)
	}

	err = templateModel.Generate()
	if err != nil {
		return fmt.Errorf("error generating from template files: %w", err)
	}

	return nil
}

func (d *Distribution) buildManifests() (string, error) {
	kzOut, err := d.kzRunner.Build()
	if err != nil {
		return "", fmt.Errorf("error building manifests: %w", err)
	}

	outDirPath, err := os.MkdirTemp("", "furyctl-dist-manifests-")
	if err != nil {
		return "", fmt.Errorf("error creating temp dir: %w", err)
	}

	manifestsOutPath := filepath.Join(outDirPath, "out.yaml")

	logrus.Debugf("built manifests = %s", manifestsOutPath)

	if err = os.WriteFile(manifestsOutPath, []byte(kzOut), os.ModePerm); err != nil {
		return "", fmt.Errorf("error writing built manifests: %w", err)
	}

	return manifestsOutPath, nil
}

func (d *Distribution) applyManifests(mPath string) error {
	err := d.delayedApplyRetries(mPath, time.Minute, kubectlDelayMaxRetry)
	if err == nil {
		return nil
	}

	err = d.delayedApplyRetries(mPath, 0, kubectlNoDelayMaxRetry)
	if err == nil {
		return nil
	}

	return err
}

func (d *Distribution) delayedApplyRetries(mPath string, delay time.Duration, maxRetries int) error {
	var err error

	retries := 0

	if maxRetries == 0 {
		return nil
	}

	err = d.kubeRunner.Apply(mPath)
	if err == nil {
		return nil
	}

	retries++

	for retries < maxRetries {
		t := time.NewTimer(delay)

		if <-t.C; true {
			logrus.Debug("applying manifests again... to ensure all resources are created.")

			err = d.kubeRunner.Apply(mPath)
			if err == nil {
				return nil
			}
		}

		retries++

		t.Stop()
	}

	return fmt.Errorf("error applying manifests: %w", err)
}
