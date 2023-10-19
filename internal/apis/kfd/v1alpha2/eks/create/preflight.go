// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package create

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"

	diffx "github.com/r3labs/diff/v3"
	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/fury-distribution/pkg/apis/ekscluster/v1alpha2/private"
	"github.com/sighupio/furyctl/configs"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/eks/rules"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/diffs"
	"github.com/sighupio/furyctl/internal/state"
	"github.com/sighupio/furyctl/internal/template"
	"github.com/sighupio/furyctl/internal/tool/awscli"
	"github.com/sighupio/furyctl/internal/tool/kubectl"
	"github.com/sighupio/furyctl/internal/tool/terraform"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

var errImmutable = errors.New("immutable path changed")

type PreFlight struct {
	*cluster.OperationPhase
	furyctlConf     private.EksclusterKfdV1Alpha2
	stateStore      state.Storer
	distroPath      string
	furyctlConfPath string
	kubeconfig      string
	tfRunner        *terraform.Runner
	kubeRunner      *kubectl.Runner
	awsRunner       *awscli.Runner
	dryRun          bool
}

func NewPreFlight(
	furyctlConf private.EksclusterKfdV1Alpha2,
	kfdManifest config.KFD,
	paths cluster.CreatorPaths,
	dryRun bool,
	stateStore state.Storer,
) (*PreFlight, error) {
	preFlightDir := path.Join(paths.WorkDir, cluster.OperationPhasePreFlight)

	phase, err := cluster.NewOperationPhase(preFlightDir, kfdManifest.Tools, paths.BinPath)
	if err != nil {
		return nil, fmt.Errorf("error creating preflight phase: %w", err)
	}

	kubeconfig := path.Join(phase.Path, "kubeconfig")

	return &PreFlight{
		OperationPhase:  phase,
		furyctlConf:     furyctlConf,
		stateStore:      stateStore,
		distroPath:      paths.DistroPath,
		furyctlConfPath: paths.ConfigPath,
		tfRunner: terraform.NewRunner(
			execx.NewStdExecutor(),
			terraform.Paths{
				Logs:      phase.TerraformLogsPath,
				Outputs:   phase.TerraformOutputsPath,
				WorkDir:   path.Join(phase.Path, "terraform"),
				Plan:      phase.TerraformPlanPath,
				Terraform: phase.TerraformPath,
			},
		),
		kubeRunner: kubectl.NewRunner(
			execx.NewStdExecutor(),
			kubectl.Paths{
				Kubectl:    phase.KubectlPath,
				WorkDir:    phase.Path,
				Kubeconfig: kubeconfig,
			},
			true,
			true,
			false,
		),
		awsRunner: awscli.NewRunner(
			execx.NewStdExecutor(),
			awscli.Paths{
				Awscli:  "aws",
				WorkDir: paths.WorkDir,
			},
		),
		kubeconfig: kubeconfig,
		dryRun:     dryRun,
	}, nil
}

func (p *PreFlight) Exec() error {
	logrus.Info("Running preflight checks")

	if err := p.CreateFolder(); err != nil {
		return fmt.Errorf("error creating preflight phase folder: %w", err)
	}

	if err := p.copyFromTemplate(); err != nil {
		return err
	}

	if err := p.CreateFolderStructure(); err != nil {
		return fmt.Errorf("error creating preflight phase folder structure: %w", err)
	}

	if err := p.tfRunner.Init(); err != nil {
		return fmt.Errorf("error running terraform init: %w", err)
	}

	if _, err := p.tfRunner.State("show", "data.aws_eks_cluster.fury"); err != nil {
		logrus.Debug("Cluster does not exist, skipping state checks")

		logrus.Info("Preflight checks completed successfully")

		return nil //nolint:nilerr // we want to return nil here
	}

	logrus.Info("Updating kubeconfig...")

	if _, err := p.awsRunner.Eks(
		"update-kubeconfig",
		"--name",
		p.furyctlConf.Metadata.Name,
		"--kubeconfig",
		p.kubeconfig,
		"--region",
		string(p.furyctlConf.Spec.Region),
	); err != nil {
		return fmt.Errorf("error updating kubeconfig: %w", err)
	}

	logrus.Info("Checking that the cluster is reachable...")

	if _, err := p.kubeRunner.Version(); err != nil {
		return fmt.Errorf("cluster is unreachable, make sure you have access to the cluster: %w", err)
	}

	diffChecker, err := p.CreateDiffChecker()
	if err != nil {
		return fmt.Errorf("error creating diff checker: %w", err)
	}

	diffs, err := p.GenerateDiffs(diffChecker)
	if err != nil {
		return fmt.Errorf("error generating diffs: %w", err)
	}

	if len(diffs) > 0 {
		logrus.Infof(
			"Differences found from previous cluster configuration:\n%s",
			diffChecker.DiffToString(diffs),
		)

		logrus.Warn("Cluster configuration has changed, checking for immutable violations...")

		if err := p.CheckStateDiffs(diffs, diffChecker); err != nil {
			return fmt.Errorf("error checking state diffs: %w", err)
		}
	}

	logrus.Info("Preflight checks completed successfully")

	return nil
}

func (p *PreFlight) copyFromTemplate() error {
	var cfg template.Config

	tmpFolder, err := os.MkdirTemp("", "furyctl-kube-configs-")
	if err != nil {
		return fmt.Errorf("error creating temp folder: %w", err)
	}

	defer os.RemoveAll(tmpFolder)

	subFS, err := fs.Sub(configs.Tpl, path.Join("provisioners", "preflight", "aws"))
	if err != nil {
		return fmt.Errorf("error getting subfs: %w", err)
	}

	err = iox.CopyRecursive(subFS, tmpFolder)
	if err != nil {
		return fmt.Errorf("error copying template files: %w", err)
	}

	targetTfDir := path.Join(p.Path, "terraform")
	prefix := "kube"

	tfConfVars := map[string]map[any]any{
		"terraform": {
			"backend": map[string]any{
				"s3": map[string]any{
					"bucketName":           p.furyctlConf.Spec.ToolsConfiguration.Terraform.State.S3.BucketName,
					"keyPrefix":            p.furyctlConf.Spec.ToolsConfiguration.Terraform.State.S3.KeyPrefix,
					"region":               p.furyctlConf.Spec.ToolsConfiguration.Terraform.State.S3.Region,
					"skipRegionValidation": p.furyctlConf.Spec.ToolsConfiguration.Terraform.State.S3.SkipRegionValidation,
				},
			},
		},
	}

	cfg.Data = tfConfVars

	err = p.OperationPhase.CopyFromTemplate(
		cfg,
		prefix,
		tmpFolder,
		targetTfDir,
		p.furyctlConfPath,
	)
	if err != nil {
		return fmt.Errorf("error generating from template files: %w", err)
	}

	return nil
}

func (p *PreFlight) CreateDiffChecker() (diffs.Checker, error) {
	storedCfg := map[string]any{}

	storedCfgStr, err := p.stateStore.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("error while getting current cluster config: %w", err)
	}

	if err = yamlx.UnmarshalV3(storedCfgStr, &storedCfg); err != nil {
		return nil, fmt.Errorf("error while unmarshalling config file: %w", err)
	}

	newCfg, err := yamlx.FromFileV3[map[string]any](p.furyctlConfPath)
	if err != nil {
		return nil, fmt.Errorf("error while reading config file: %w", err)
	}

	return diffs.NewBaseChecker(storedCfg, newCfg), nil
}

func (*PreFlight) GenerateDiffs(diffChecker diffs.Checker) (diffx.Changelog, error) {
	diffs, err := diffChecker.GenerateDiff()
	if err != nil {
		return nil, fmt.Errorf("error while diffing configs: %w", err)
	}

	return diffs, nil
}

func (p *PreFlight) CheckStateDiffs(diffs diffx.Changelog, diffChecker diffs.Checker) error {
	var errs []error

	r, err := rules.NewEKSClusterRulesBuilder(p.distroPath)
	if err != nil {
		if !errors.Is(err, rules.ErrReadingRulesFile) {
			return fmt.Errorf("error while creating rules builder: %w", err)
		}

		logrus.Warn("No rules file found, skipping immutable checks")

		return nil
	}

	errs = append(errs, diffChecker.AssertImmutableViolations(diffs, r.GetImmutables("infrastructure"))...)
	errs = append(errs, diffChecker.AssertImmutableViolations(diffs, r.GetImmutables("kubernetes"))...)
	errs = append(errs, diffChecker.AssertImmutableViolations(diffs, r.GetImmutables("distribution"))...)

	if len(errs) > 0 {
		return fmt.Errorf("%w: %s", errImmutable, errs)
	}

	return nil
}
