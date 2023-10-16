// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package create

import (
	"errors"
	"fmt"
	"path"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/fury-distribution/pkg/apis/onpremises/v1alpha2/public"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/onpremises/rules"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/diff"
	"github.com/sighupio/furyctl/internal/state"
	"github.com/sighupio/furyctl/internal/tool/kubectl"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

var errImmutable = errors.New("immutable path changed")

type PreFlight struct {
	*cluster.OperationPhase
	furyctlConf     public.OnpremisesKfdV1Alpha2
	stateStore      state.Storer
	distroPath      string
	furyctlConfPath string
	kubeconfig      string
	kubeRunner      *kubectl.Runner
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

	// TODO: Check that cluster exists, ansible needed

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

	if err := p.CheckStateDiffs(storedCfg); err != nil {
		return fmt.Errorf("error checking state diffs: %w", err)
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

func (p *PreFlight) CheckStateDiffs(storedCfgStr []byte) error {
	var errs []error

	storedCfg := map[string]any{}

	if err := yamlx.UnmarshalV3(storedCfgStr, &storedCfg); err != nil {
		return fmt.Errorf("error while unmarshalling config file: %w", err)
	}

	newCfg, err := yamlx.FromFileV3[map[string]any](p.furyctlConfPath)
	if err != nil {
		return fmt.Errorf("error while reading config file: %w", err)
	}

	diffChecker := diff.NewBaseChecker(storedCfg, newCfg)

	diffs, err := diffChecker.GenerateDiff()
	if err != nil {
		return fmt.Errorf("error while diffing configs: %w", err)
	}

	logrus.Debug("Diff: ", diffs)

	r, err := rules.NewOnPremClusterRulesBuilder(p.distroPath)
	if err != nil {
		return fmt.Errorf("error while creating rules builder: %w", err)
	}

	errs = append(errs, diffChecker.AssertImmutableViolations(diffs, r.GetImmutables("kubernetes"))...)
	errs = append(errs, diffChecker.AssertImmutableViolations(diffs, r.GetImmutables("distribution"))...)

	if len(errs) > 0 {
		return fmt.Errorf("%w: %s", errImmutable, errs)
	}

	return nil
}
