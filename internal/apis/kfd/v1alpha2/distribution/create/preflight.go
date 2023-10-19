// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package create

import (
	"errors"
	"fmt"
	"path"

	diffx "github.com/r3labs/diff/v3"
	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/fury-distribution/pkg/apis/kfddistribution/v1alpha2/public"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/distribution/rules"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/diffs"
	"github.com/sighupio/furyctl/internal/state"
	"github.com/sighupio/furyctl/internal/tool/kubectl"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

var errImmutable = errors.New("immutable path changed")

type PreFlight struct {
	*cluster.OperationPhase
	furyctlConf     public.KfddistributionKfdV1Alpha2
	stateStore      state.Storer
	distroPath      string
	furyctlConfPath string
	kubeconfig      string
	kubeRunner      *kubectl.Runner
	dryRun          bool
}

func NewPreFlight(
	furyctlConf public.KfddistributionKfdV1Alpha2,
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
		stateStore:      stateStore,
		distroPath:      paths.DistroPath,
		furyctlConfPath: paths.ConfigPath,
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
		kubeconfig: kubeconfig,
		dryRun:     dryRun,
	}, nil
}

func (p *PreFlight) Exec() error {
	logrus.Info("Running preflight checks")

	logrus.Info("Checking that the cluster is reachable...")

	if _, err := p.kubeRunner.Version(); err != nil {
		return fmt.Errorf("cluster is unreachable, make sure you have access to the cluster: %w", err)
	}

	storedCfg, err := p.GetStateFromCluster()
	if err != nil {
		logrus.Debug("error while getting cluster state: ", err)

		logrus.Info("Cannot find state in cluster, skipping...")

		logrus.Debug("check that the secret exists in the cluster if you want to run preflight checks")

		return nil
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

	r, err := rules.NewDistroClusterRulesBuilder(p.distroPath)
	if err != nil {
		if !errors.Is(err, rules.ErrReadingRulesFile) {
			return fmt.Errorf("error while creating rules builder: %w", err)
		}

		logrus.Warn("No rules file found, skipping immutable checks")

		return nil
	}

	errs = append(errs, diffChecker.AssertImmutableViolations(d, r.GetImmutables("distribution"))...)

	if len(errs) > 0 {
		return fmt.Errorf("%w: %s", errImmutable, errs)
	}

	return nil
}
