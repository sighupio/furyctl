// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package create

import (
	"fmt"
	"os"
	"path"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/fury-distribution/pkg/apis/onpremises/v1alpha2/public"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/state"
	"github.com/sighupio/furyctl/internal/tool/kubectl"
	"github.com/sighupio/furyctl/internal/tool/shell"
	"github.com/sighupio/furyctl/internal/upgrade"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
	"github.com/sighupio/furyctl/pkg/template"
	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

const (
	LifecyclePreApply  = "pre-apply"
	LifecyclePostApply = "post-apply"
)

type Distribution struct {
	*cluster.OperationPhase

	furyctlConfPath string
	furyctlConf     public.OnpremisesKfdV1Alpha2
	kfdManifest     config.KFD
	paths           cluster.CreatorPaths
	stateStore      state.Storer
	dryRun          bool
	shellRunner     *shell.Runner
	kubeRunner      *kubectl.Runner
	upgrade         *upgrade.Upgrade
}

func (d *Distribution) Self() *cluster.OperationPhase {
	return d.OperationPhase
}

func (d *Distribution) Exec(reducers v1alpha2.Reducers, startFrom string, upgradeState *upgrade.State) error {
	logrus.Info("Installing Kubernetes Fury Distribution...")

	mCfg, err := d.prepare()
	if err != nil {
		return fmt.Errorf("error preparing distribution phase: %w", err)
	}

	if d.dryRun {
		logrus.Info("Kubernetes Fury Distribution installed successfully (dry-run mode)")

		return nil
	}

	if err := d.preDistribution(startFrom, upgradeState); err != nil {
		return fmt.Errorf("error running pre-distribution phase: %w", err)
	}

	if err := d.coreDistribution(startFrom, upgradeState, reducers, mCfg); err != nil {
		return fmt.Errorf("error running core distribution phase: %w", err)
	}

	if err := d.postDistribution(upgradeState); err != nil {
		return fmt.Errorf("error running post-distribution phase: %w", err)
	}

	logrus.Info("Kubernetes Fury Distribution installed successfully")

	return nil
}

func (d *Distribution) prepare() (template.Config, error) {
	if err := d.CreateRootFolder(); err != nil {
		return template.Config{}, fmt.Errorf("error creating distribution phase folder: %w", err)
	}

	if _, err := os.Stat(path.Join(d.OperationPhase.Path, "manifests")); os.IsNotExist(err) {
		if err := os.Mkdir(path.Join(d.OperationPhase.Path, "manifests"), iox.FullPermAccess); err != nil {
			return template.Config{}, fmt.Errorf("error creating manifests folder: %w", err)
		}
	}

	furyctlMerger, err := d.CreateFuryctlMerger(
		d.paths.DistroPath,
		d.paths.ConfigPath,
		"kfd-v1alpha2",
		"onpremises",
	)
	if err != nil {
		return template.Config{}, fmt.Errorf("error creating furyctl merger: %w", err)
	}

	mCfg, err := template.NewConfigWithoutData(furyctlMerger, []string{"terraform", ".gitignore", "manifests/aws"})
	if err != nil {
		return template.Config{}, fmt.Errorf("error creating template config: %w", err)
	}

	d.CopyPathsToConfig(&mCfg)

	// Check cluster connection and requirements.
	storageClassAvailable := true

	logrus.Info("Checking that the cluster is reachable...")

	if _, err := d.kubeRunner.Version(); err != nil {
		logrus.Debugf("Got error while running cluster reachability check: %s", err)

		return template.Config{}, fmt.Errorf("error connecting to cluster: %w", err)
	}

	logrus.Info("Checking storage classes...")

	getStorageClassesOutput, err := d.kubeRunner.Get(false, "", "storageclasses")
	if err != nil {
		return template.Config{}, fmt.Errorf("error while checking storage class: %w", err)
	}

	if getStorageClassesOutput == "No resources found" {
		logrus.Warn(
			"No storage classes found in the cluster. " +
				"logging module (if enabled), tracing module (if enabled), dr module (if enabled) " +
				"and prometheus-operated package installation will be skipped. " +
				"You need to install a StorageClass and re-run furyctl to install the missing components.",
		)

		storageClassAvailable = false
	}

	mCfg.Data["checks"] = map[any]any{
		"storageClassAvailable": storageClassAvailable,
	}

	mCfg, err = d.injectStoredConfig(mCfg)
	if err != nil {
		return template.Config{}, fmt.Errorf("error injecting stored config: %w", err)
	}

	if err := d.CopyFromTemplate(
		mCfg,
		"distribution",
		path.Join(d.paths.DistroPath, "templates", cluster.OperationPhaseDistribution),
		d.Path,
		d.paths.ConfigPath,
	); err != nil {
		return template.Config{}, fmt.Errorf("error copying from template: %w", err)
	}

	return mCfg, nil
}

func (d *Distribution) preDistribution(
	startFrom string,
	upgradeState *upgrade.State,
) error {
	if startFrom == "" || startFrom == cluster.OperationSubPhasePreDistribution {
		// Run upgrade script if needed.
		if err := d.upgrade.Exec(d.Path, "pre-distribution"); err != nil {
			upgradeState.Phases.PreDistribution.Status = upgrade.PhaseStatusFailed

			return fmt.Errorf("error running upgrade: %w", err)
		}

		if d.upgrade.Enabled {
			upgradeState.Phases.PreDistribution.Status = upgrade.PhaseStatusSuccess
		}
	}

	return nil
}

func (d *Distribution) coreDistribution(
	startFrom string,
	upgradeState *upgrade.State,
	reducers v1alpha2.Reducers,
	mCfg template.Config,
) error {
	if startFrom != cluster.OperationSubPhasePostDistribution {
		logrus.Info("Applying manifests...")

		if err := d.runReducers(
			reducers,
			mCfg,
			LifecyclePreApply,
			[]string{"manifests", "terraform", ".gitignore"},
		); err != nil {
			return fmt.Errorf("error running pre-apply reducers: %w", err)
		}

		if _, err := d.shellRunner.Run(path.Join(d.Path, "scripts", "apply.sh")); err != nil {
			if d.upgrade.Enabled {
				upgradeState.Phases.Distribution.Status = upgrade.PhaseStatusFailed
			}

			return fmt.Errorf("error applying manifests: %w", err)
		}

		if d.upgrade.Enabled {
			upgradeState.Phases.Distribution.Status = upgrade.PhaseStatusSuccess
		}

		if err := d.runReducers(
			reducers,
			mCfg,
			LifecyclePostApply,
			[]string{"manifests", "terraform", ".gitignore"},
		); err != nil {
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

		if err := d.CopyFromTemplate(
			preTfReducersCfg,
			"distribution",
			path.Join(d.paths.DistroPath, "templates", cluster.OperationPhaseDistribution),
			d.Path,
			d.paths.ConfigPath,
		); err != nil {
			return fmt.Errorf("error copying from template: %w", err)
		}

		if _, err := d.shellRunner.Run(
			path.Join(d.Path, "scripts", lifecycle+".sh"),
		); err != nil {
			return fmt.Errorf("error applying manifests: %w", err)
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

func NewDistribution(
	furyctlConf public.OnpremisesKfdV1Alpha2,
	kfdManifest config.KFD,
	paths cluster.CreatorPaths,
	dryRun bool,
	upgr *upgrade.Upgrade,
) *Distribution {
	phase := cluster.NewOperationPhase(
		path.Join(paths.WorkDir, cluster.OperationPhaseDistribution),
		kfdManifest.Tools,
		paths.BinPath,
	)

	return &Distribution{
		OperationPhase:  phase,
		furyctlConf:     furyctlConf,
		kfdManifest:     kfdManifest,
		paths:           paths,
		dryRun:          dryRun,
		furyctlConfPath: paths.ConfigPath,
		stateStore: state.NewStore(
			paths.DistroPath,
			paths.ConfigPath,
			paths.WorkDir,
			kfdManifest.Tools.Common.Kubectl.Version,
			paths.BinPath,
		),
		shellRunner: shell.NewRunner(
			execx.NewStdExecutor(),
			shell.Paths{
				Shell:   "sh",
				WorkDir: path.Join(phase.Path, "manifests"),
			},
		),
		kubeRunner: kubectl.NewRunner(
			execx.NewStdExecutor(),
			kubectl.Paths{
				Kubectl: phase.KubectlPath,
				WorkDir: path.Join(phase.Path, "manifests"),
			},
			true,
			true,
			false,
		),
		upgrade: upgr,
	}
}
