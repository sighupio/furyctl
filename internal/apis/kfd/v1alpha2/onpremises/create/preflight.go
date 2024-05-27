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
	"github.com/sighupio/fury-distribution/pkg/apis/onpremises/v1alpha2/public"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/onpremises/rules"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/onpremises/supported"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/state"
	"github.com/sighupio/furyctl/internal/tool/ansible"
	"github.com/sighupio/furyctl/internal/tool/kubectl"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	kubex "github.com/sighupio/furyctl/internal/x/kube"
	"github.com/sighupio/furyctl/pkg/diffs"
	"github.com/sighupio/furyctl/pkg/template"
	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

var (
	errImmutable   = errors.New("immutable path changed")
	errUnsupported = errors.New("unsupported reducer values detected")
)

type Status struct {
	Diffs   r3diff.Changelog
	Success bool
}

type PreFlight struct {
	*cluster.OperationPhase

	furyctlConf    public.OnpremisesKfdV1Alpha2
	paths          cluster.CreatorPaths
	stateStore     state.Storer
	kubeRunner     *kubectl.Runner
	ansibleRunner  *ansible.Runner
	kfdManifest    config.KFD
	dryRun         bool
	force          []string
	phase          string
	upgradeEnabled bool
}

func NewPreFlight(
	furyctlConf public.OnpremisesKfdV1Alpha2,
	kfdManifest config.KFD,
	paths cluster.CreatorPaths,
	dryRun bool,
	stateStore state.Storer,
	force []string,
	phase string,
	upgradeEnabled bool,
) *PreFlight {
	p := cluster.NewOperationPhase(
		path.Join(paths.WorkDir, cluster.OperationPhasePreFlight),
		kfdManifest.Tools,
		paths.BinPath,
	)

	return &PreFlight{
		OperationPhase: p,
		furyctlConf:    furyctlConf,
		paths:          paths,
		stateStore:     stateStore,
		ansibleRunner: ansible.NewRunner(
			execx.NewStdExecutor(),
			ansible.Paths{
				Ansible:         "ansible",
				AnsiblePlaybook: "ansible-playbook",
				WorkDir:         p.Path,
			},
		),
		kubeRunner: kubectl.NewRunner(
			execx.NewStdExecutor(),
			kubectl.Paths{
				Kubectl: p.KubectlPath,
				WorkDir: p.Path,
			},
			true,
			true,
			false,
		),
		kfdManifest:    kfdManifest,
		dryRun:         dryRun,
		force:          force,
		phase:          phase,
		upgradeEnabled: upgradeEnabled,
	}
}

func (p *PreFlight) Exec(renderedConfig map[string]any) (*Status, error) {
	status := &Status{
		Diffs:   r3diff.Changelog{},
		Success: false,
	}

	logrus.Info("Running preflight checks")

	if err := p.CreateRootFolder(); err != nil {
		return status, fmt.Errorf("error creating kubernetes phase folder: %w", err)
	}

	furyctlMerger, err := p.CreateFuryctlMerger(
		p.paths.DistroPath,
		p.paths.ConfigPath,
		"kfd-v1alpha2",
		"onpremises",
	)
	if err != nil {
		return status, fmt.Errorf("error creating furyctl merger: %w", err)
	}

	mCfg, err := template.NewConfigWithoutData(furyctlMerger, []string{})
	if err != nil {
		return status, fmt.Errorf("error creating template config: %w", err)
	}

	mCfg.Data["kubernetes"] = map[any]any{
		"version": p.kfdManifest.Kubernetes.OnPremises.Version,
	}

	if err := p.CopyFromTemplate(
		mCfg,
		"preflight",
		path.Join(p.paths.DistroPath, "templates", cluster.OperationPhasePreFlight, "onpremises"),
		p.Path,
		p.paths.ConfigPath,
	); err != nil {
		return status, fmt.Errorf("error copying from template: %w", err)
	}

	if _, err := p.ansibleRunner.Exec("all", "-m", "ping"); err != nil {
		return status, fmt.Errorf("error checking hosts: %w", err)
	}

	if _, err := p.ansibleRunner.Playbook("verify-playbook.yaml"); err != nil {
		status.Success = true

		logrus.Debug("Cluster does not exist, skipping state checks")

		logrus.Info("Preflight checks completed successfully")

		return status, nil //nolint:nilerr // we want to return nil here
	}

	if err := kubex.SetConfigEnv(path.Join(p.Path, "admin.conf")); err != nil {
		return status, fmt.Errorf("error setting kubeconfig env: %w", err)
	}

	logrus.Info("Checking that the cluster is reachable...")

	if _, err := p.kubeRunner.Version(); err != nil {
		return status, fmt.Errorf("cluster is unreachable, make sure you have access to the cluster: %w", err)
	}

	diffChecker, err := p.CreateDiffChecker(renderedConfig)
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

			if p.phase != cluster.OperationPhaseAll && !p.upgradeEnabled {
				logrus.Info("Cluster configuration has changed, checking if changes are supported in the current phase...")

				if err := cluster.AssertPhaseDiffs(d, p.phase, (&supported.Phases{}).Get()); err != nil {
					return status, fmt.Errorf("error checking changes to other phases: %w", err)
				}
			}
		}
	}

	logrus.Info("Preflight checks completed successfully")

	status.Success = true

	return status, nil
}

func (p *PreFlight) CreateDiffChecker(renderedConfig map[string]any) (diffs.Checker, error) {
	clusterCfg := map[string]any{}

	storedCfgStr, err := p.stateStore.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("error while getting current cluster config: %w", err)
	}

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

	r, err := rules.NewOnPremClusterRulesExtractor(p.paths.DistroPath)
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

func (p *PreFlight) CheckReducerDiffs(d r3diff.Changelog, diffChecker diffs.Checker) error {
	var errs []error

	r, err := rules.NewOnPremClusterRulesExtractor(p.paths.DistroPath)
	if err != nil {
		if !errors.Is(err, rules.ErrReadingRulesFile) {
			return fmt.Errorf("error while creating rules builder: %w", err)
		}

		logrus.Warn("No rules file found, skipping reducer checks")

		return nil
	}

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
