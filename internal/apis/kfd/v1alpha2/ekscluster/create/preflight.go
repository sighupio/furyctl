// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package create

import (
	"errors"
	"fmt"
	"path"

	r3diff "github.com/r3labs/diff/v3"
	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/fury-distribution/pkg/apis/ekscluster/v1alpha2/private"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/ekscluster/common"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/ekscluster/rules"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/ekscluster/vpn"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/diffs"
	"github.com/sighupio/furyctl/internal/state"
	"github.com/sighupio/furyctl/internal/tool/awscli"
	"github.com/sighupio/furyctl/internal/tool/kubectl"
	"github.com/sighupio/furyctl/internal/tool/terraform"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	kubex "github.com/sighupio/furyctl/internal/x/kube"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

var (
	errImmutable   = errors.New("immutable path changed")
	errUnsupported = errors.New("unsupported reducer values detected")
)

type Status struct {
	Diffs   r3diff.Changelog
	Success bool
}

// Preflight is a phase tasked with ensuring cluster connectivity
// and checking for violations in the updates made on the furyctl.yaml file.
type PreFlight struct {
	*common.PreFlight

	stateStore   state.Storer
	tfRunnerKube *terraform.Runner

	kubeRunner *kubectl.Runner
	dryRun     bool
	paths      cluster.CreatorPaths
	force      []string
}

func NewPreFlight(
	furyctlConf private.EksclusterKfdV1Alpha2,
	kfdManifest config.KFD,
	paths cluster.CreatorPaths,
	dryRun bool,
	vpnAutoConnect bool,
	skipVpn bool,
	force []string,
	infraOutputsPath string,
) (*PreFlight, error) {
	phase := cluster.NewOperationPhase(
		path.Join(paths.WorkDir, cluster.OperationPhasePreFlight),
		kfdManifest.Tools,
		paths.BinPath,
	)

	var vpnConfig *private.SpecInfrastructureVpn
	if furyctlConf.Spec.Infrastructure != nil {
		vpnConfig = furyctlConf.Spec.Infrastructure.Vpn
	}

	vpnConnector, err := vpn.NewConnector(
		furyctlConf.Metadata.Name,
		path.Join(phase.Path, "secrets"),
		paths.BinPath,
		kfdManifest.Tools.Common.Furyagent.Version,
		vpnAutoConnect,
		skipVpn,
		vpnConfig,
	)
	if err != nil {
		return nil, fmt.Errorf("error while creating vpn connector: %w", err)
	}

	return &PreFlight{
		PreFlight: &common.PreFlight{
			OperationPhase: phase,
			FuryctlConf:    furyctlConf,
			ConfigPath:     paths.ConfigPath,
			AWSRunner: awscli.NewRunner(
				execx.NewStdExecutor(),
				awscli.Paths{
					Awscli:  "aws",
					WorkDir: paths.WorkDir,
				},
			),
			VPNConnector: vpnConnector,
			TFRunnerInfra: terraform.NewRunner(
				execx.NewStdExecutor(),
				terraform.Paths{
					WorkDir:   path.Join(phase.Path, "terraform", "infrastructure"),
					Terraform: phase.TerraformPath,
				},
			),
			InfrastructureTerraformOutputsPath: infraOutputsPath,
		},
		stateStore: state.NewStore(
			paths.DistroPath,
			paths.ConfigPath,
			paths.WorkDir,
			kfdManifest.Tools.Common.Kubectl.Version,
			paths.BinPath,
		),
		tfRunnerKube: terraform.NewRunner(
			execx.NewStdExecutor(),
			terraform.Paths{
				WorkDir:   path.Join(phase.Path, "terraform", "kubernetes"),
				Terraform: phase.TerraformPath,
			},
		),
		kubeRunner: kubectl.NewRunner(
			execx.NewStdExecutor(),
			kubectl.Paths{
				Kubectl: phase.KubectlPath,
				WorkDir: phase.Path,
			},
			true,
			true,
			false,
		),
		dryRun: dryRun,
		paths:  paths,
		force:  force,
	}, nil
}

func (p *PreFlight) Exec(renderedConfig map[string]any) (*Status, error) {
	status := &Status{
		Diffs:   r3diff.Changelog{},
		Success: false,
	}

	logrus.Info("Ensure prerequisites are in place...")

	if err := p.EnsureTerraformStateAWSS3Bucket(); err != nil {
		return status, fmt.Errorf("error ensuring terraform state aws s3 bucket: %w", err)
	}

	logrus.Info("Running preflight checks")

	if err := p.Prepare(); err != nil {
		return status, fmt.Errorf("error preparing preflight phase: %w", err)
	}

	if err := p.tfRunnerKube.Init(); err != nil {
		return status, fmt.Errorf("error running terraform init: %w", err)
	}

	if _, err := p.tfRunnerKube.State("show", "data.aws_eks_cluster.fury"); err != nil {
		logrus.Debug("Cluster does not exist, skipping state checks")

		logrus.Info("Preflight checks completed successfully")

		status.Success = true

		return status, nil //nolint:nilerr // we want to return nil here
	}

	kubeconfig := path.Join(p.Path, "secrets", "kubeconfig")

	logrus.Info("Updating kubeconfig...")

	if _, err := p.AWSRunner.Eks(
		false,
		"update-kubeconfig",
		"--name",
		p.FuryctlConf.Metadata.Name,
		"--kubeconfig",
		kubeconfig,
		"--region",
		string(p.FuryctlConf.Spec.Region),
	); err != nil {
		return status, fmt.Errorf("error updating kubeconfig: %w", err)
	}

	if err := kubex.SetConfigEnv(kubeconfig); err != nil {
		return status, fmt.Errorf("error setting kubeconfig env: %w", err)
	}

	if p.IsVPNRequired() {
		if err := p.HandleVPN(); err != nil {
			return status, fmt.Errorf("error handling vpn: %w", err)
		}
	}

	logrus.Info("Checking that the cluster is reachable...")

	if _, err := p.kubeRunner.Version(); err != nil {
		return status, fmt.Errorf("cluster is unreachable, make sure you have access to the cluster: %w", err)
	}

	storedCfg, err := p.stateStore.GetConfig()
	if err != nil {
		logrus.Debug("error while getting cluster state: ", err)

		logrus.Info("Cannot find state in cluster, skipping...")

		logrus.Debug("check that the secret exists in the cluster if you want to run preflight checks")

		return status, nil
	}

	diffChecker, err := p.CreateDiffChecker(storedCfg, renderedConfig)
	if err != nil {
		if !cluster.IsForceEnabledForFeature(p.force, cluster.ForceFeatureMigrations) {
			return status, fmt.Errorf(
				"error creating diff checker: %w; "+
					"if this happened after a failed attempt at creating a cluster, retry using the \"--force migrations\" flag",
				err,
			)
		}

		logrus.Error("error creating diff checker, skipping: %w", err)
	} else {
		d, err := diffChecker.GenerateDiff()
		if err != nil {
			return status, fmt.Errorf("error while generating diff: %w", err)
		}

		status.Diffs = d

		if len(d) > 0 {
			logrus.Debugf(
				"Differences found from previous cluster configuration:\n%s",
				diffChecker.DiffToString(d),
			)

			logrus.Info("Cluster configuration has changed, checking for immutable violations...")

			if err := p.CheckImmutablesDiffs(d, diffChecker); err != nil {
				return status, fmt.Errorf("error checking state diffs: %w", err)
			}

			logrus.Info("Cluster configuration has changed, checking for unsupported reducers violations...")

			if err := p.CheckReducersDiffs(d, diffChecker); err != nil {
				return status, fmt.Errorf("error checking reducer diffs: %w", err)
			}
		}
	}

	logrus.Info("Preflight checks completed successfully")

	status.Success = true

	return status, nil
}

func (p *PreFlight) CreateDiffChecker(
	storedCfgStr []byte,
	renderedConfig map[string]any,
) (diffs.Checker, error) {
	clusterCfg := map[string]any{}

	clusterRenderedCfg, err := p.stateStore.GetRenderedConfig()
	if err == nil {
		if err := yamlx.UnmarshalV3(clusterRenderedCfg, &clusterCfg); err != nil {
			return nil, fmt.Errorf("error while unmarshalling rendered config file: %w", err)
		}

		return diffs.NewBaseChecker(clusterCfg, renderedConfig), nil
	}

	if err := yamlx.UnmarshalV3(storedCfgStr, &clusterCfg); err != nil {
		return nil, fmt.Errorf("error while unmarshalling config file: %w", err)
	}

	cfg, err := yamlx.FromFileV3[map[string]any](p.paths.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("error while reading config file: %w", err)
	}

	return diffs.NewBaseChecker(clusterCfg, cfg), nil
}

// CheckImmutablesDiffs checks if there have been changes to immutable fields in the furyctl.yaml.
func (p *PreFlight) CheckImmutablesDiffs(d r3diff.Changelog, diffChecker diffs.Checker) error {
	var errs []error

	r, err := rules.NewEKSClusterRulesExtractor(p.paths.DistroPath)
	if err != nil {
		if !errors.Is(err, rules.ErrReadingRulesFile) {
			return fmt.Errorf("error while creating rules builder: %w", err)
		}

		logrus.Warn("No rules file found, skipping immutable checks")

		return nil
	}

	errs = append(errs, diffChecker.AssertImmutableViolations(d, r.GetImmutables("infrastructure"))...)
	errs = append(errs, diffChecker.AssertImmutableViolations(d, r.GetImmutables("kubernetes"))...)
	errs = append(errs, diffChecker.AssertImmutableViolations(d, r.GetImmutables("distribution"))...)

	if len(errs) > 0 {
		return fmt.Errorf("%w: %s", errImmutable, errs)
	}

	return nil
}

// CheckReducersDiffs checks if the changes to the reducers are supported by the distribution.
// This is needed as not all from/to combinations are supported.
func (p *PreFlight) CheckReducersDiffs(d r3diff.Changelog, diffChecker diffs.Checker) error {
	var errs []error

	r, err := rules.NewEKSClusterRulesExtractor(p.paths.DistroPath)
	if err != nil {
		if !errors.Is(err, rules.ErrReadingRulesFile) {
			return fmt.Errorf("error while creating rules builder: %w", err)
		}

		logrus.Warn("No rules file found, skipping reducer checks")

		return nil
	}

	errs = append(errs, diffChecker.AssertReducerUnsupportedViolations(
		d,
		r.UnsupportedReducerRulesByDiffs(r.GetReducers("infrastructure"), d),
	)...)
	errs = append(errs, diffChecker.AssertReducerUnsupportedViolations(
		d,
		r.UnsupportedReducerRulesByDiffs(r.GetReducers("kubernetes"), d),
	)...)
	errs = append(errs, diffChecker.AssertReducerUnsupportedViolations(
		d,
		r.UnsupportedReducerRulesByDiffs(r.GetReducers("distribution"), d),
	)...)

	if len(errs) > 0 {
		return fmt.Errorf("%w: %s", errUnsupported, errs)
	}

	return nil
}
