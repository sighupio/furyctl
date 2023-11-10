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

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/fury-distribution/pkg/apis/ekscluster/v1alpha2/private"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/merge"
	"github.com/sighupio/furyctl/internal/state"
	"github.com/sighupio/furyctl/internal/template"
	"github.com/sighupio/furyctl/internal/tool/kubectl"
	"github.com/sighupio/furyctl/internal/tool/shell"
	"github.com/sighupio/furyctl/internal/tool/terraform"
	"github.com/sighupio/furyctl/internal/upgrade"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

const (
	LifecyclePreTf     = "pre-tf"
	LifecyclePostTf    = "post-tf"
	LifecyclePreApply  = "pre-apply"
	LifecyclePostApply = "post-apply"
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
	stateStore       state.Storer
	tfRunner         *terraform.Runner
	shellRunner      *shell.Runner
	kubeRunner       *kubectl.Runner
	dryRun           bool
	phase            string
	kubeconfig       string
	upgrade          *upgrade.Upgrade
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
	kubeconfig string,
	upgrade *upgrade.Upgrade,
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
		stateStore: state.NewStore(
			paths.DistroPath,
			paths.ConfigPath,
			kubeconfig,
			paths.WorkDir,
			kfdManifest.Tools.Common.Kubectl.Version,
			paths.BinPath,
		),
		tfRunner: terraform.NewRunner(
			execx.NewStdExecutor(),
			terraform.Paths{
				Logs:      phaseOp.TerraformLogsPath,
				Outputs:   phaseOp.TerraformOutputsPath,
				WorkDir:   path.Join(phaseOp.Path, "terraform"),
				Plan:      phaseOp.TerraformPlanPath,
				Terraform: phaseOp.TerraformPath,
			},
		),
		shellRunner: shell.NewRunner(
			execx.NewStdExecutor(),
			shell.Paths{
				Shell:   "sh",
				WorkDir: path.Join(phaseOp.Path, "manifests"),
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
		dryRun:     dryRun,
		phase:      phase,
		kubeconfig: kubeconfig,
		upgrade:    upgrade,
	}, nil
}

func (*Distribution) SupportsLifecycle(lifecycle string) bool {
	switch lifecycle {
	case LifecyclePreTf, LifecyclePostTf, LifecyclePreApply, LifecyclePostApply:
		return true

	default:
		return false
	}
}

func (d *Distribution) Exec(reducers v1alpha2.Reducers) error {
	timestamp := time.Now().Unix()

	logrus.Info("Installing Kubernetes Fury Distribution...")

	logrus.Debug("Create: running distribution phase...")

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

	tfCfg, err := template.NewConfig(furyctlMerger, preTfMerger, []string{"manifests", "scripts", ".gitignore"})
	if err != nil {
		return fmt.Errorf("error creating template config: %w", err)
	}

	if err := d.copyFromTemplate(tfCfg); err != nil {
		return err
	}

	if err := d.CreateFolderStructure(); err != nil {
		return fmt.Errorf("error creating distribution phase folder structure: %w", err)
	}

	tfCfg, err = d.injectStoredConfig(tfCfg)
	if err != nil {
		return fmt.Errorf("error injecting stored config: %w", err)
	}

	if err := d.runReducers(reducers, tfCfg, LifecyclePreTf, []string{"manifests", ".gitignore"}); err != nil {
		return fmt.Errorf("error running pre-tf reducers: %w", err)
	}

	if err := d.tfRunner.Init(); err != nil {
		return fmt.Errorf("error running terraform init: %w", err)
	}

	if _, err := d.tfRunner.Plan(timestamp); err != nil && !d.dryRun {
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

		mCfg.Data["paths"] = map[any]any{
			"kubectl":   d.OperationPhase.KubectlPath,
			"kustomize": d.OperationPhase.KustomizePath,
			"yq":        d.OperationPhase.YqPath,
		}

		mCfg.Data["checks"] = map[any]any{
			"storageClassAvailable": true,
		}

		if err := d.copyFromTemplate(mCfg); err != nil {
			return err
		}

		if _, err := d.shellRunner.Run(path.Join(d.Path, "scripts", "apply.sh"), "true", d.kubeconfig); err != nil {
			return fmt.Errorf("error applying manifests: %w", err)
		}

		return nil
	}

	// Run upgrade script if needed.
	if err := d.upgrade.Exec(d.Path, "pre-distribution"); err != nil {
		return fmt.Errorf("error running upgrade: %w", err)
	}

	logrus.Warn("Creating cloud resources, this could take a while...")

	if err := d.tfRunner.Apply(timestamp); err != nil {
		return fmt.Errorf("cannot create cloud resources: %w", err)
	}

	if _, err := d.tfRunner.Output(); err != nil {
		return fmt.Errorf("error running terraform output: %w", err)
	}

	postTfMerger, err := d.injectDataPostTf(preTfMerger)
	if err != nil {
		return err
	}

	mCfg, err := template.NewConfig(furyctlMerger, postTfMerger, []string{"terraform", ".gitignore"})
	if err != nil {
		return fmt.Errorf("error creating template config: %w", err)
	}

	mCfg.Data["paths"] = map[any]any{
		"kubectl":    d.KubectlPath,
		"kustomize":  d.KustomizePath,
		"yq":         d.YqPath,
		"vendorPath": path.Join(d.Path, "..", "vendor"),
	}

	mCfg.Data["checks"] = map[any]any{
		"storageClassAvailable": true,
	}

	mCfg, err = d.injectStoredConfig(mCfg)
	if err != nil {
		return fmt.Errorf("error injecting stored config: %w", err)
	}

	if err := d.copyFromTemplate(mCfg); err != nil {
		return err
	}

	if err := d.runReducers(reducers, mCfg, LifecyclePostTf, []string{"manifests", ".gitignore"}); err != nil {
		return fmt.Errorf("error running post-tf reducers: %w", err)
	}

	logrus.Info("Checking that the cluster is reachable...")

	if err := d.checkKubeVersion(); err != nil {
		return fmt.Errorf("error checking cluster reachability: %w", err)
	}

	if err := d.runReducers(reducers, mCfg, LifecyclePreApply, []string{"manifests", ".gitignore"}); err != nil {
		return fmt.Errorf("error running pre-apply reducers: %w", err)
	}

	logrus.Info("Applying manifests...")

	if _, err := d.shellRunner.Run(path.Join(d.Path, "scripts", "apply.sh"), "false", d.kubeconfig); err != nil {
		return fmt.Errorf("error applying manifests: %w", err)
	}

	if err := d.runReducers(reducers, mCfg, LifecyclePostApply, []string{"manifests", ".gitignore"}); err != nil {
		return fmt.Errorf("error running post-apply reducers: %w", err)
	}

	// Run upgrade script if needed.
	if err := d.upgrade.Exec(d.Path, "post-distribution"); err != nil {
		return fmt.Errorf("error running upgrade: %w", err)
	}

	return nil
}

func (d *Distribution) checkKubeVersion() error {
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

	return nil
}

func (d *Distribution) injectStoredConfig(cfg template.Config) (template.Config, error) {
	storedCfg := map[any]any{}

	storedCfgStr, err := d.stateStore.GetConfig()
	if err != nil {
		logrus.Debugf("error while getting current config, skipping stored config injection: %s", err)

		return cfg, nil
	}

	if err = yamlx.UnmarshalV3(storedCfgStr, &storedCfg); err != nil {
		return cfg, fmt.Errorf("error while unmarshalling config file: %w", err)
	}

	cfg.Data["storedCfg"] = storedCfg

	return cfg, nil
}

func (d *Distribution) runReducers(
	reducers v1alpha2.Reducers,
	cfg template.Config,
	lifecycle string,
	excludes []string,
) error {
	r := reducers.ByLifecycle(lifecycle)

	if len(r) > 0 {
		preTfReducersCfg := cfg
		preTfReducersCfg.Data = r.Combine(cfg.Data, "reducers")
		preTfReducersCfg.Templates.Excludes = excludes

		if err := d.copyFromTemplate(preTfReducersCfg); err != nil {
			return err
		}

		if _, err := d.shellRunner.Run(
			path.Join(d.Path, "scripts", fmt.Sprintf("%s.sh", lifecycle)),
			"true",
			d.kubeconfig,
		); err != nil {
			return fmt.Errorf("error applying manifests: %w", err)
		}
	}

	return nil
}

func (d *Distribution) Stop() error {
	errCh := make(chan error)
	doneCh := make(chan bool)

	var wg sync.WaitGroup

	//nolint:gomnd,revive // ignore magic number linters
	wg.Add(3)

	go func() {
		logrus.Debug("Stopping terraform...")

		if err := d.tfRunner.Stop(); err != nil {
			errCh <- fmt.Errorf("error stopping terraform: %w", err)
		}

		wg.Done()
	}()

	go func() {
		logrus.Debug("Stopping shell...")

		if err := d.shellRunner.Stop(); err != nil {
			errCh <- fmt.Errorf("error stopping shell: %w", err)
		}

		wg.Done()
	}()

	go func() {
		logrus.Debug("Stopping kubectl...")

		if err := d.kubeRunner.Stop(); err != nil {
			errCh <- fmt.Errorf("error stopping kubectl: %w", err)
		}

		wg.Done()
	}()

	go func() {
		wg.Wait()
		close(doneCh)
	}()

	select {
	case <-doneCh:

	case err := <-errCh:
		close(errCh)

		return err
	}

	return nil
}

func (d *Distribution) createFuryctlMerger() (*merge.Merger, error) {
	defaultsFilePath := path.Join(d.distroPath, "defaults", "ekscluster-kfd-v1alpha2.yaml")

	defaultsFile, err := yamlx.FromFileV2[map[any]any](defaultsFilePath)
	if err != nil {
		return &merge.Merger{}, fmt.Errorf("%s - %w", defaultsFilePath, err)
	}

	furyctlConf, err := yamlx.FromFileV2[map[any]any](d.furyctlConfPath)
	if err != nil {
		return &merge.Merger{}, fmt.Errorf("%s - %w", d.furyctlConfPath, err)
	}

	merger := merge.NewMerger(
		merge.NewDefaultModel(defaultsFile, ".data"),
		merge.NewDefaultModel(furyctlConf, ".spec.distribution"),
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
			if infraOut["vpc_id"] == nil {
				return vpcID, ErrVpcIDNotFound
			}

			vpcIDOut, ok := infraOut["vpc_id"].Value.(string)
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
			},
		},
	}

	if d.furyctlConf.Spec.Distribution.Modules.Dr.Type == "eks" {
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

	outputFilePath := path.Join(d.TerraformOutputsPath, "output.json")

	if _, err := os.Stat(outputFilePath); err == nil {
		return nil
	}

	if err := os.MkdirAll(d.TerraformOutputsPath, iox.FullPermAccess); err != nil {
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
	arns := map[string]string{}

	out, err := d.tfRunner.Output()
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
		d.furyctlConfPath,
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
