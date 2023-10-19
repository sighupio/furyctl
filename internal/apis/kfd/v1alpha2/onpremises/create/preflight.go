// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package create

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"

	diffx "github.com/r3labs/diff/v3"
	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/fury-distribution/pkg/apis/onpremises/v1alpha2/public"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/onpremises/rules"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/diffs"
	"github.com/sighupio/furyctl/internal/merge"
	"github.com/sighupio/furyctl/internal/state"
	"github.com/sighupio/furyctl/internal/template"
	"github.com/sighupio/furyctl/internal/tool/ansible"
	"github.com/sighupio/furyctl/internal/tool/kubectl"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	kubex "github.com/sighupio/furyctl/internal/x/kube"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

var errImmutable = errors.New("immutable path changed")

type PreFlight struct {
	*cluster.OperationPhase
	furyctlConf     public.OnpremisesKfdV1Alpha2
	paths           cluster.CreatorPaths
	stateStore      state.Storer
	distroPath      string
	furyctlConfPath string
	kubeconfig      string
	kubeRunner      *kubectl.Runner
	ansibleRunner   *ansible.Runner
	kfdManifest     config.KFD
	dryRun          bool
}

func NewPreFlight(
	furyctlConf public.OnpremisesKfdV1Alpha2,
	kfdManifest config.KFD,
	paths cluster.CreatorPaths,
	dryRun bool,
	kubeconfig string,
	stateStore state.Storer,
) (*PreFlight, error) {
	preFlightDir := path.Join(paths.WorkDir, cluster.OperationPhasePreFlight)

	phase, err := cluster.NewOperationPhase(preFlightDir, kfdManifest.Tools, paths.BinPath)
	if err != nil {
		return nil, fmt.Errorf("error creating preflight phase: %w", err)
	}

	return &PreFlight{
		OperationPhase:  phase,
		furyctlConf:     furyctlConf,
		paths:           paths,
		stateStore:      stateStore,
		distroPath:      paths.DistroPath,
		furyctlConfPath: paths.ConfigPath,
		ansibleRunner: ansible.NewRunner(
			execx.NewStdExecutor(),
			ansible.Paths{
				Ansible:         "ansible",
				AnsiblePlaybook: "ansible-playbook",
				WorkDir:         phase.Path,
			},
		),
		kubeRunner: kubectl.NewRunner(
			execx.NewStdExecutor(),
			kubectl.Paths{
				Kubectl:    phase.KubectlPath,
				WorkDir:    phase.Path,
				Kubeconfig: paths.Kubeconfig,
			},
			true,
			true,
			false,
		),
		kubeconfig:  kubeconfig,
		kfdManifest: kfdManifest,
		dryRun:      dryRun,
	}, nil
}

func (p *PreFlight) Exec() error {
	logrus.Info("Running preflight checks")

	if err := p.CreateFolder(); err != nil {
		return fmt.Errorf("error creating kubernetes phase folder: %w", err)
	}

	furyctlMerger, err := p.createFuryctlMerger()
	if err != nil {
		return err
	}

	mCfg, err := template.NewConfigWithoutData(furyctlMerger, []string{})
	if err != nil {
		return fmt.Errorf("error creating template config: %w", err)
	}

	mCfg.Data["kubernetes"] = map[any]any{
		"version": p.kfdManifest.Kubernetes.OnPremises.Version,
	}

	// Generate playbook and hosts file.
	outYaml, err := yamlx.MarshalV2(mCfg)
	if err != nil {
		return fmt.Errorf("error marshaling template config: %w", err)
	}

	outDirPath1, err := os.MkdirTemp("", "furyctl-dist-")
	if err != nil {
		return fmt.Errorf("error creating temp dir: %w", err)
	}

	confPath := filepath.Join(outDirPath1, "config.yaml")

	logrus.Debugf("config path = %s", confPath)

	if err = os.WriteFile(confPath, outYaml, os.ModePerm); err != nil {
		return fmt.Errorf("error writing config file: %w", err)
	}

	templateModel, err := template.NewTemplateModel(
		path.Join(p.paths.DistroPath, "templates", cluster.OperationPhasePreFlight, "onpremises"),
		path.Join(p.Path),
		confPath,
		outDirPath1,
		p.furyctlConfPath,
		".tpl",
		false,
		p.dryRun,
	)
	if err != nil {
		return fmt.Errorf("error creating template model: %w", err)
	}

	err = templateModel.Generate()
	if err != nil {
		return fmt.Errorf("error generating from template files: %w", err)
	}

	if _, err := p.ansibleRunner.Exec("all", "-m", "ping"); err != nil {
		return fmt.Errorf("error checking hosts: %w", err)
	}

	if _, err := p.ansibleRunner.Playbook("verify-playbook.yaml"); err != nil {
		logrus.Debug("Cluster does not exist, skipping state checks")

		logrus.Info("Preflight checks completed successfully")

		return nil //nolint:nilerr // we want to return nil here
	}

	if err := kubex.SetConfigEnv(path.Join(p.OperationPhase.Path, "admin.conf")); err != nil {
		return fmt.Errorf("error setting kubeconfig env: %w", err)
	}

	logrus.Info("Checking that the cluster is reachable...")

	if _, err := p.kubeRunner.Version(); err != nil {
		return fmt.Errorf("cluster is unreachable, make sure you have access to the cluster: %w", err)
	}

	storedCfg, err := p.GetStateFromCluster()
	if err != nil {
		return fmt.Errorf("error while getting current cluster config: %w", err)
	}

	diffChecker, err := p.CreateDiffChecker(storedCfg)
	if err != nil {
		return fmt.Errorf("error creating diff checker: %w", err)
	}

	d, err := diffChecker.GenerateDiff()
	if err != nil {
		return fmt.Errorf("error while generating diff: %w", err)
	}

	if len(d) > 0 {
		logrus.Infof(
			"Differences found from previous cluster configuration:\n%s",
			diffChecker.DiffToString(d),
		)

		logrus.Warn("Cluster configuration has changed, checking for immutable violations...")

		if err := p.CheckStateDiffs(d, diffChecker); err != nil {
			return fmt.Errorf("error checking state diffs: %w", err)
		}
	}

	logrus.Info("Preflight checks completed successfully")

	return nil
}

//nolint:dupl // Remove duplicated code in the future.
func (p *PreFlight) createFuryctlMerger() (*merge.Merger, error) {
	defaultsFilePath := path.Join(p.paths.DistroPath, "defaults", "onpremises-kfd-v1alpha2.yaml")

	defaultsFile, err := yamlx.FromFileV2[map[any]any](defaultsFilePath)
	if err != nil {
		return &merge.Merger{}, fmt.Errorf("%s - %w", defaultsFilePath, err)
	}

	furyctlConf, err := yamlx.FromFileV2[map[any]any](p.paths.ConfigPath)
	if err != nil {
		return &merge.Merger{}, fmt.Errorf("%s - %w", p.paths.ConfigPath, err)
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

func (p *PreFlight) GetStateFromCluster() ([]byte, error) {
	storedCfgStr, err := p.stateStore.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("error while getting current cluster config: %w", err)
	}

	return storedCfgStr, nil
}

func (p *PreFlight) CreateDiffChecker(storedCfgStr []byte) (diffs.Checker, error) {
	storedCfg := map[string]any{}

	if err := yamlx.UnmarshalV3(storedCfgStr, &storedCfg); err != nil {
		return nil, fmt.Errorf("error while unmarshalling config file: %w", err)
	}

	newCfg, err := yamlx.FromFileV3[map[string]any](p.furyctlConfPath)
	if err != nil {
		return nil, fmt.Errorf("error while reading config file: %w", err)
	}

	return diffs.NewBaseChecker(storedCfg, newCfg), nil
}

func (p *PreFlight) CheckStateDiffs(d diffx.Changelog, diffChecker diffs.Checker) error {
	var errs []error

	r, err := rules.NewOnPremClusterRulesExtractor(p.distroPath)
	if err != nil {
		if !errors.Is(err, rules.ErrReadingRulesFile) {
			return fmt.Errorf("error while creating rules builder: %w", err)
		}

		logrus.Warn("No rules file found, skipping immutable checks")

		return nil
	}

	errs = append(errs, diffChecker.AssertImmutableViolations(d, r.GetImmutables("kubernetes"))...)
	errs = append(errs, diffChecker.AssertImmutableViolations(d, r.GetImmutables("distribution"))...)

	if len(errs) > 0 {
		return fmt.Errorf("%w: %s", errImmutable, errs)
	}

	return nil
}
