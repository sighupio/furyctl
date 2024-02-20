// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package create

import (
	"errors"
	"fmt"
	"os"
	"path"

	r3diff "github.com/r3labs/diff/v3"
	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/fury-distribution/pkg/apis/kfddistribution/v1alpha2/public"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/kfddistribution/rules"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/diffs"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/parser"
	"github.com/sighupio/furyctl/internal/state"
	"github.com/sighupio/furyctl/internal/tool/kubectl"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	kubex "github.com/sighupio/furyctl/internal/x/kube"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

var (
	errImmutable        = errors.New("immutable path changed")
	errUnsupported      = errors.New("unsupported reducer values detected")
	ErrKubeconfigNotSet = errors.New("KUBECONFIG env variable is not set")
)

type Status struct {
	Diffs   r3diff.Changelog
	Success bool
}

type PreFlight struct {
	*cluster.OperationPhase
	furyctlConf     public.KfddistributionKfdV1Alpha2
	stateStore      state.Storer
	distroPath      string
	furyctlConfPath string
	kubeRunner      *kubectl.Runner
	paths           cluster.CreatorPaths
	kfd             config.KFD
	dryRun          bool
	force           []string
}

func NewPreFlight(
	furyctlConf public.KfddistributionKfdV1Alpha2,
	kfdManifest config.KFD,
	paths cluster.CreatorPaths,
	dryRun bool,
	stateStore state.Storer,
	force []string,
) *PreFlight {
	phase := cluster.NewOperationPhase(
		path.Join(paths.WorkDir, cluster.OperationPhasePreFlight),
		kfdManifest.Tools,
		paths.BinPath,
	)

	return &PreFlight{
		OperationPhase:  phase,
		furyctlConf:     furyctlConf,
		stateStore:      stateStore,
		distroPath:      paths.DistroPath,
		furyctlConfPath: paths.ConfigPath,
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
		paths:  paths,
		kfd:    kfdManifest,
		dryRun: dryRun,
		force:  force,
	}
}

func (p *PreFlight) Exec(renderedConfig map[string]any) (*Status, error) {
	var err error

	status := &Status{
		Diffs:   r3diff.Changelog{},
		Success: false,
	}

	cfgParser := parser.NewConfigParser(p.furyctlConfPath)

	logrus.Info("Running preflight checks")

	if err := p.CreateRootFolder(); err != nil {
		return status, fmt.Errorf("error creating preflight phase folder: %w", err)
	}

	kubeconfigPath := os.Getenv("KUBECONFIG")

	if distribution.HasFeature(p.kfd, distribution.FeatureKubeconfigInSchema) {
		kubeconfigPath, err = cfgParser.ParseDynamicValue(p.furyctlConf.Spec.Distribution.Kubeconfig)
		if err != nil {
			return status, fmt.Errorf("error parsing kubeconfig value: %w", err)
		}
	} else if kubeconfigPath == "" {
		return status, ErrKubeconfigNotSet
	}

	if err := kubex.SetConfigEnv(kubeconfigPath); err != nil {
		return status, fmt.Errorf("error setting kubeconfig env: %w", err)
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

			if err := p.CheckStateDiffs(d, diffChecker); err != nil {
				return status, fmt.Errorf("error checking state diffs: %w", err)
			}

			logrus.Info("Cluster configuration has changed, checking for unsupported reducers violations...")

			if err := p.CheckReducerDiffs(d, diffChecker); err != nil {
				return status, fmt.Errorf("error checking reducer diffs: %w", err)
			}
		}
	}

	status.Success = true

	logrus.Info("Preflight checks completed successfully")

	return status, nil
}

func (p *PreFlight) CreateDiffChecker(storedCfgStr []byte, renderedConfig map[string]any) (diffs.Checker, error) {
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

func (p *PreFlight) CheckStateDiffs(d r3diff.Changelog, diffChecker diffs.Checker) error {
	var errs []error

	r, err := rules.NewDistroClusterRulesExtractor(p.distroPath)
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

func (p *PreFlight) CheckReducerDiffs(d r3diff.Changelog, diffChecker diffs.Checker) error {
	var errs []error

	r, err := rules.NewDistroClusterRulesExtractor(p.distroPath)
	if err != nil {
		if !errors.Is(err, rules.ErrReadingRulesFile) {
			return fmt.Errorf("error while creating rules builder: %w", err)
		}

		logrus.Warn("No rules file found, skipping reducer checks")

		return nil
	}

	errs = append(errs, diffChecker.AssertReducerUnsupportedViolations(
		d,
		r.UnsupportedReducerRulesByDiffs(r.GetReducers("distribution"), d),
	)...)

	if len(errs) > 0 {
		return fmt.Errorf("%w: %s", errUnsupported, errs)
	}

	return nil
}
