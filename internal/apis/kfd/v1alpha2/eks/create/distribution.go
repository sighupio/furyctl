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
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/fury-distribution/pkg/apis/ekscluster/v1alpha2/private"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/eks/common"
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
	*common.Distribution

	furyctlConf private.EksclusterKfdV1Alpha2
	kfdManifest config.KFD
	stateStore  state.Storer
	tfRunner    *terraform.Runner
	shellRunner *shell.Runner
	kubeRunner  *kubectl.Runner
	phase       string
	upgrade     *upgrade.Upgrade
	paths       cluster.CreatorPaths
}

type injectType struct {
	Data private.SpecDistribution `json:"data"`
}

func NewDistribution(
	paths cluster.CreatorPaths,
	furyctlConf private.EksclusterKfdV1Alpha2,
	kfdManifest config.KFD,
	dryRun bool,
	phase string,
	upgr *upgrade.Upgrade,
) *Distribution {
	phaseOp := cluster.NewOperationPhase(
		path.Join(paths.WorkDir, cluster.OperationPhaseDistribution),
		kfdManifest.Tools,
		paths.BinPath,
	)

	return &Distribution{
		Distribution: &common.Distribution{
			OperationPhase: phaseOp,
			DryRun:         dryRun,
			DistroPath:     paths.DistroPath,
			ConfigPath:     paths.ConfigPath,
		},
		furyctlConf: furyctlConf,
		kfdManifest: kfdManifest,
		stateStore: state.NewStore(
			paths.DistroPath,
			paths.ConfigPath,
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
				Kubectl: phaseOp.KubectlPath,
				WorkDir: path.Join(phaseOp.Path, "manifests"),
			},
			true,
			true,
			false,
		),
		phase:   phase,
		upgrade: upgr,
		paths:   paths,
	}
}

func (d *Distribution) Self() *cluster.OperationPhase {
	return d.OperationPhase
}

func (*Distribution) SupportsLifecycle(lifecycle string) bool {
	switch lifecycle {
	case LifecyclePreTf, LifecyclePostTf, LifecyclePreApply, LifecyclePostApply:
		return true

	default:
		return false
	}
}

func (d *Distribution) Exec(
	reducers v1alpha2.Reducers,
	startFrom string,
	upgradeState *upgrade.State,
) error {
	timestamp := time.Now().Unix()

	logrus.Info("Installing Kubernetes Fury Distribution...")

	furyctlMerger, preTfMerger, tfCfg, err := d.Prepare()
	if err != nil {
		return fmt.Errorf("error preparing distribution phase: %w", err)
	}

	if err = d.injectStoredConfig(tfCfg); err != nil {
		return fmt.Errorf("error injecting stored config: %w", err)
	}

	if err := d.tfRunner.Init(); err != nil {
		return fmt.Errorf("error running terraform init: %w", err)
	}

	if err := d.preDistribution(startFrom, upgradeState); err != nil {
		return fmt.Errorf("error running pre-distribution phase: %w", err)
	}

	if err := d.coreDistribution(
		reducers,
		tfCfg,
		startFrom,
		upgradeState,
		preTfMerger,
		furyctlMerger,
		timestamp,
	); err != nil {
		return fmt.Errorf("error running core distribution phase: %w", err)
	}

	if d.DryRun {
		logrus.Info("Kubernetes Fury Distribution installed successfully (dry-run mode)")

		return nil
	}

	if err := d.postDistribution(upgradeState); err != nil {
		return fmt.Errorf("error running post-distribution phase: %w", err)
	}

	logrus.Info("Kubernetes Fury Distribution installed successfully")

	return nil
}

func (d *Distribution) preDistribution(
	startFrom string,
	upgradeState *upgrade.State,
) error {
	if !d.DryRun {
		if startFrom == "" || startFrom == cluster.OperationSubPhasePreDistribution {
			if err := d.upgrade.Exec(d.Path, "pre-distribution"); err != nil {
				upgradeState.Phases.PreDistribution.Status = upgrade.PhaseStatusFailed

				return fmt.Errorf("error running upgrade: %w", err)
			}

			if d.upgrade.Enabled {
				upgradeState.Phases.PreDistribution.Status = upgrade.PhaseStatusSuccess
			}
		}
	}

	return nil
}

func (d *Distribution) coreDistribution(
	reducers v1alpha2.Reducers,
	tfCfg *template.Config,
	startFrom string,
	upgradeState *upgrade.State,
	preTfMerger *merge.Merger,
	furyctlMerger *merge.Merger,
	timestamp int64,
) error {
	if startFrom != cluster.OperationSubPhasePostDistribution {
		if err := d.runReducers(reducers, tfCfg, LifecyclePreTf, []string{"manifests", ".gitignore"}); err != nil {
			return fmt.Errorf("error running pre-tf reducers: %w", err)
		}

		if _, err := d.tfRunner.Plan(timestamp); err != nil && !d.DryRun {
			return fmt.Errorf("error running terraform plan: %w", err)
		}

		if d.DryRun {
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

			d.CopyPathsToConfig(&mCfg)

			mCfg.Data["checks"] = map[any]any{
				"storageClassAvailable": true,
			}

			if err := d.CopyFromTemplate(
				mCfg,
				"distribution",
				path.Join(d.paths.DistroPath, "templates", cluster.OperationPhaseDistribution),
				d.Path,
				d.paths.ConfigPath,
			); err != nil {
				return fmt.Errorf("error copying from template: %w", err)
			}

			return nil
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

		d.CopyPathsToConfig(&mCfg)

		mCfg.Data["checks"] = map[any]any{
			"storageClassAvailable": true,
		}

		if err = d.injectStoredConfig(&mCfg); err != nil {
			return fmt.Errorf("error injecting stored config: %w", err)
		}

		if err := d.CopyFromTemplate(
			mCfg,
			"distribution",
			path.Join(d.paths.DistroPath, "templates", cluster.OperationPhaseDistribution),
			d.Path,
			d.paths.ConfigPath,
		); err != nil {
			return fmt.Errorf("error copying from template: %w", err)
		}

		if err := d.runReducers(reducers, &mCfg, LifecyclePostTf, []string{"manifests", ".gitignore"}); err != nil {
			return fmt.Errorf("error running post-tf reducers: %w", err)
		}

		logrus.Info("Checking that the cluster is reachable...")

		if err := d.checkKubeVersion(); err != nil {
			return fmt.Errorf("error checking cluster reachability: %w", err)
		}

		if err := d.runReducers(reducers, &mCfg, LifecyclePreApply, []string{"manifests", ".gitignore"}); err != nil {
			return fmt.Errorf("error running pre-apply reducers: %w", err)
		}

		logrus.Info("Applying manifests...")

		if _, err := d.shellRunner.Run(path.Join(d.Path, "scripts", "apply.sh")); err != nil {
			if d.upgrade.Enabled {
				upgradeState.Phases.Distribution.Status = upgrade.PhaseStatusFailed
			}

			return fmt.Errorf("error applying manifests: %w", err)
		}

		if d.upgrade.Enabled {
			upgradeState.Phases.Distribution.Status = upgrade.PhaseStatusSuccess
		}

		if err := d.runReducers(reducers, &mCfg, LifecyclePostApply, []string{"manifests", ".gitignore"}); err != nil {
			return fmt.Errorf("error running post-apply reducers: %w", err)
		}
	}

	return nil
}

func (d *Distribution) postDistribution(
	upgradeState *upgrade.State,
) error {
	if err := d.upgrade.Exec(d.Path, "post-distribution"); err != nil {
		upgradeState.Phases.PostDistribution.Status = upgrade.PhaseStatusFailed

		return fmt.Errorf("error running upgrade: %w", err)
	}

	if d.upgrade.Enabled {
		upgradeState.Phases.PostDistribution.Status = upgrade.PhaseStatusSuccess
	}

	return nil
}

func (d *Distribution) checkKubeVersion() error {
	if _, err := d.kubeRunner.Version(); err != nil {
		logrus.Debugf("Got error while running cluster reachability check: %s", err)

		if !d.DryRun {
			return errClusterConnect
		}

		if d.phase == cluster.OperationPhaseDistribution {
			logrus.Warnf("Cluster is unreachable, make sure it is reachable before " +
				"running the command without --dry-run")
		}
	}

	return nil
}

func (d *Distribution) injectStoredConfig(cfg *template.Config) error {
	storedCfg := map[any]any{}

	storedCfgStr, err := d.stateStore.GetConfig()
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

func (d *Distribution) runReducers(
	reducers v1alpha2.Reducers,
	cfg *template.Config,
	lifecycle string,
	excludes []string,
) error {
	r := reducers.ByLifecycle(lifecycle)

	if len(r) > 0 {
		preTfReducersCfg := cfg
		preTfReducersCfg.Data = r.Combine(cfg.Data, "reducers")
		preTfReducersCfg.Templates.Excludes = excludes

		if err := d.CopyFromTemplate(
			*preTfReducersCfg,
			"distribution",
			path.Join(d.paths.DistroPath, "templates", cluster.OperationPhaseDistribution),
			d.Path,
			d.paths.ConfigPath,
		); err != nil {
			return fmt.Errorf("error copying from template: %w", err)
		}

		if _, err := d.shellRunner.Run(
			path.Join(d.Path, "scripts", fmt.Sprintf("%s.sh", lifecycle)),
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
