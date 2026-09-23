// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package create

import (
	"errors"
	"fmt"
	"os"
	"path"
	"slices"
	"strings"

	r3diff "github.com/r3labs/diff/v3"
	"github.com/sirupsen/logrus"

	"github.com/sighupio/furyctl/internal/apis/config"
	preflightx "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/immutable/preflight"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/immutable/public"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/immutable/supported"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/state"
	"github.com/sighupio/furyctl/internal/tool/ansible"
	"github.com/sighupio/furyctl/internal/tool/kubectl"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	kubex "github.com/sighupio/furyctl/internal/x/kube"
	"github.com/sighupio/furyctl/pkg/diffs"
	rules "github.com/sighupio/furyctl/pkg/rulesextractor"
	templatex "github.com/sighupio/furyctl/pkg/template"
	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

var (
	errImmutable   = errors.New("immutable path changed")
	errUnsupported = errors.New("unsupported reducer values detected")
)

type Status struct {
	Diffs         r3diff.Changelog
	Success       bool
	ClusterExists bool
}

type PreFlight struct {
	*cluster.OperationPhase

	furyctlConf    public.ImmutableKfdV1Alpha2
	paths          cluster.CreatorPaths
	stateStore     state.Storer
	kubeRunner     *kubectl.Runner
	ansibleRunner  *ansible.Runner
	kfdManifest    config.KFD
	dryRun         bool
	force          []string
	phase          string
	startFrom      string
	upgradeEnabled bool
}

func NewPreFlight(
	furyctlConf public.ImmutableKfdV1Alpha2,
	kfdManifest config.KFD,
	paths cluster.CreatorPaths,
	dryRun bool,
	stateStore state.Storer,
	force []string,
	phase string,
	startFrom string,
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
			ansible.PathsForVersion(paths.BinPath, kfdManifest.Tools.Immutable.Ansible.Version, p.Path),
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
		startFrom:      startFrom,
		upgradeEnabled: upgradeEnabled,
	}
}

func (p *PreFlight) Exec(renderedConfig map[string]any) (*Status, error) {
	status := &Status{
		Diffs:         r3diff.Changelog{},
		Success:       false,
		ClusterExists: false,
	}

	logrus.Info("Running preflight checks...")

	if err := p.CreateRootFolder(); err != nil {
		return status, fmt.Errorf("error creating preflight phase folder: %w", err)
	}

	furyctlMerger, err := p.CreateFuryctlMerger(
		p.paths.DistroPath,
		p.paths.ConfigPath,
		"kfd-v1alpha2",
		"immutable",
	)
	if err != nil {
		return status, fmt.Errorf("error creating furyctl merger: %w", err)
	}

	mCfg, err := templatex.NewConfigWithoutData(furyctlMerger, []string{})
	if err != nil {
		return status, fmt.Errorf("error creating template config: %w", err)
	}

	mCfg.Data["kubernetes"] = map[any]any{
		"version": p.kfdManifest.Kubernetes.Immutable.Version,
	}

	if err := p.CopyFromTemplate(
		mCfg,
		"preflight",
		path.Join(p.paths.DistroPath, "templates", cluster.OperationPhasePreFlight, "immutable"),
		p.Path,
		p.paths.ConfigPath,
	); err != nil {
		return status, fmt.Errorf("error copying from template: %w", err)
	}

	adminConfPath := path.Join(p.Path, "admin.conf")

	adminConfPlaybook, err := preflightx.AdminConfPlaybookName(p.Path)
	if err != nil {
		return status, fmt.Errorf("error selecting admin.conf playbook: %w", err)
	}

	// The phase folder holds the files of an earlier run, because CreateRootFolder keeps a
	// folder that exists. A cluster that the operator removed would therefore read as a cluster
	// that is there. Remove the files, so that only the playbook below can put them back.
	for _, f := range []string{adminConfPath, path.Join(p.Path, clusterStateFile)} {
		if err := os.Remove(f); err != nil && !errors.Is(err, os.ErrNotExist) {
			return status, fmt.Errorf("error removing %s of an earlier run: %w", path.Base(f), err)
		}
	}

	_, playbookErr := p.ansibleRunner.Playbook(adminConfPlaybook)

	probe, hasProbe, err := readClusterState(p.Path)
	if err != nil {
		return status, err
	}

	switch {
	case hasProbe && playbookErr != nil:
		// This playbook does not fail when a host does not answer, so its error is a true error.
		return status, fmt.Errorf("error checking the hosts of the cluster: %w", playbookErr)

	case hasProbe:
		logrus.Info(probe.summary())

		warning, err := probe.assess(p.controlPlaneHosts())
		if err != nil {
			return status, err
		}

		if warning != "" {
			logrus.Warn(warning)
		}

	case playbookErr != nil:
		// A distribution released before the state file gives a playbook that fails when a host
		// does not answer. This kind creates its machines in the infrastructure phase, so a first
		// apply cannot reach the control plane hosts, and an error here is the normal answer for
		// a cluster that is not there yet. The run therefore continues, and the check below gives
		// the same answer as before. What keeps the run away from another cluster is the gate of
		// the creator, which stops a phase that reads a cluster when there is none.
		logrus.Warnf(
			"furyctl could not read these control plane hosts: %s. It continues as if the cluster "+
				"does not exist. A first apply gives this message, because the infrastructure phase "+
				"creates those hosts. If the cluster is there, stop furyctl and make sure that these "+
				"hosts answer.",
			strings.Join(p.controlPlaneHosts(), ", "),
		)

		logrus.Debugf("%s: %v", adminConfPlaybook, playbookErr)

	default:
		// A distribution released before the state file gives a playbook that ran without an
		// error, so every control plane host answered. The check below reads admin.conf.
	}

	// The playbook fetches admin.conf when a control plane node holds one, and it does not
	// fail when no node holds one. The local file therefore answers the question, which
	// keeps an error of the playbook an error.
	if _, err := os.Stat(adminConfPath); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return status, fmt.Errorf("error reading the kubeconfig of the cluster: %w", err)
		}

		status.Success = true

		logrus.Debug("Cluster does not exist, skipping state checks")

		logrus.Info("Preflight checks completed successfully")

		return status, nil
	}

	if err := kubex.SetConfigEnv(adminConfPath); err != nil {
		return status, fmt.Errorf("error setting kubeconfig env: %w", err)
	}

	// ClusterExists comes after KUBECONFIG points to the cluster that the check found.
	status.ClusterExists = true

	logrus.Info("Checking that the Kubernetes API is reachable...")

	if _, err := p.kubeRunner.Version(); err != nil {
		return status, fmt.Errorf("cluster is unreachable, make sure you have access to the cluster: %w", err)
	}

	diffChecker, err := p.CreateDiffChecker(renderedConfig)
	if err != nil {
		if !cluster.IsForceEnabledForFeature(p.force, cluster.ForceFeatureMigrations) {
			return status, fmt.Errorf(
				"error creating configuration diff checker: %w; "+
					"if this happened after a failed attempt at creating a cluster, retry using the \"--force migrations\" flag",
				err,
			)
		}

		logrus.WithError(err).Warn("error creating configuration diff checker but force flag was used. Continuing")
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

			logrus.Info("Cluster configuration has changed, checking for immutability violations...")

			if err := p.CheckStateDiffs(d, diffChecker); err != nil {
				return status, fmt.Errorf("error checking state diffs: %w", err)
			}

			logrus.Info("Cluster configuration has changed, checking for unsupported reducers violations...")

			if err := p.CheckReducerDiffs(d, diffChecker); err != nil {
				return status, fmt.Errorf("error checking reducer diffs: %w", err)
			}

			if (p.phase != cluster.OperationPhaseAll || p.startFrom != cluster.OperationPhaseAll) && !p.upgradeEnabled {
				logrus.Info("Cluster configuration has changed, checking if changes are supported in the phases to apply...")

				if err := cluster.AssertPhaseDiffs(d, p.phase, p.startFrom, supported.Phases()); err != nil {
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
	r, err := rules.NewImmutableClusterRulesExtractor(
		p.paths.DistroPath,
		diffChecker.GetCurrentConfig(),
		supported.Phases(),
	)
	if err != nil {
		if !errors.Is(err, rules.ErrReadingRulesFile) {
			return fmt.Errorf("error while creating rules builder: %w", err)
		}

		logrus.Warn("No rules file found, skipping immutable checks")

		return nil
	}

	// Extract the paths from the rules left after filtering out the ones with matching safe conditions.
	filteredInfraImmutableRules := r.FilterSafeImmutableRules(r.GetImmutableRules("infrastructure"), d)
	filteredKubeImmutableRules := r.FilterSafeImmutableRules(r.GetImmutableRules("kubernetes"), d)
	filteredDistroImmutableRules := r.FilterSafeImmutableRules(r.GetImmutableRules("distribution"), d)

	infraImmutablePaths := rules.Paths(filteredInfraImmutableRules)
	kubeImmutablePaths := rules.Paths(filteredKubeImmutableRules)
	distroImmutablePaths := rules.Paths(filteredDistroImmutableRules)

	errs := slices.Concat(
		diffChecker.AssertImmutableViolations(d, infraImmutablePaths),
		diffChecker.AssertImmutableViolations(d, kubeImmutablePaths),
		diffChecker.AssertImmutableViolations(d, distroImmutablePaths),
	)

	if len(errs) > 0 {
		return fmt.Errorf("%w: %w", errImmutable, errors.Join(errs...))
	}

	return nil
}

func (p *PreFlight) CheckReducerDiffs(d r3diff.Changelog, diffChecker diffs.Checker) error {
	r, err := rules.NewImmutableClusterRulesExtractor(
		p.paths.DistroPath,
		diffChecker.GetCurrentConfig(),
		supported.Phases(),
	)
	if err != nil {
		if !errors.Is(err, rules.ErrReadingRulesFile) {
			return fmt.Errorf("error while creating rules builder: %w", err)
		}

		logrus.Warn("No rules file found, skipping reducer checks")

		return nil
	}

	errs := slices.Concat(
		diffChecker.AssertReducerUnsupportedViolations(
			d,
			r.GetUnsupportedRules(cluster.OperationPhaseInfrastructure),
		),
		diffChecker.AssertReducerUnsupportedViolations(
			d,
			r.GetUnsupportedRules(cluster.OperationPhaseKubernetes),
		),
		diffChecker.AssertReducerUnsupportedViolations(
			d,
			r.GetUnsupportedRules(cluster.OperationPhaseDistribution),
		),
	)

	if len(errs) > 0 {
		return fmt.Errorf("%w: %w", errUnsupported, errors.Join(errs...))
	}

	return nil
}

// controlPlaneHosts gives the hosts that this check probes. Its inventory holds the
// control plane group only.
func (p *PreFlight) controlPlaneHosts() []string {
	hosts := make([]string, 0, len(p.furyctlConf.Spec.Kubernetes.ControlPlane.Members))

	for _, m := range p.furyctlConf.Spec.Kubernetes.ControlPlane.Members {
		hosts = append(hosts, m.Hostname)
	}

	return hosts
}
