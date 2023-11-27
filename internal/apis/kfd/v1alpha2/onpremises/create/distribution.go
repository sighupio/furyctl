// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package create

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/fury-distribution/pkg/apis/onpremises/v1alpha2/public"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/merge"
	"github.com/sighupio/furyctl/internal/state"
	"github.com/sighupio/furyctl/internal/template"
	"github.com/sighupio/furyctl/internal/tool/kubectl"
	"github.com/sighupio/furyctl/internal/tool/shell"
	"github.com/sighupio/furyctl/internal/upgrade"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
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
	upgradeStore    upgrade.Storer
	upgrade         *upgrade.Upgrade
}

func (d *Distribution) Exec(reducers v1alpha2.Reducers, startFrom string, upgradeState *upgrade.State) error {
	logrus.Info("Installing Kubernetes Fury Distribution...")
	logrus.Debug("Create: running distribution phase...")

	if err := d.CreateFolder(); err != nil {
		return fmt.Errorf("error creating distribution phase folder: %w", err)
	}

	if _, err := os.Stat(path.Join(d.OperationPhase.Path, "manifests")); os.IsNotExist(err) {
		err = os.Mkdir(path.Join(d.OperationPhase.Path, "manifests"), iox.FullPermAccess)
		if err != nil {
			return fmt.Errorf("error creating manifests folder: %w", err)
		}
	}

	furyctlMerger, err := d.createFuryctlMerger()
	if err != nil {
		return err
	}

	mCfg, err := template.NewConfigWithoutData(furyctlMerger, []string{"terraform", ".gitignore", "manifests/aws"})
	if err != nil {
		return fmt.Errorf("error creating template config: %w", err)
	}

	d.CopyPathsToConfig(&mCfg)

	// Check cluster connection and requirements.
	storageClassAvailable := true

	logrus.Info("Checking that the cluster is reachable...")

	if _, err := d.kubeRunner.Version(); err != nil {
		logrus.Debugf("Got error while running cluster reachability check: %s", err)

		return fmt.Errorf("error connecting to cluster: %w", err)
	}

	logrus.Info("Checking storage classes...")

	getStorageClassesOutput, err := d.kubeRunner.Get(false, "", "storageclasses")
	if err != nil {
		return fmt.Errorf("error while checking storage class: %w", err)
	}

	if getStorageClassesOutput == "No resources found" {
		logrus.Warn(
			"No storage classes found in the cluster. " +
				"logging module (if enabled), dr module (if enabled) " +
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
		return fmt.Errorf("error injecting stored config: %w", err)
	}

	// Generate manifests.
	if err := d.copyFromTemplate(mCfg); err != nil {
		return fmt.Errorf("error generating from template files: %w", err)
	}

	// Stop if dry run is enabled.
	if d.dryRun {
		logrus.Info("Kubernetes Fury Distribution installed successfully (dry-run mode)")

		return nil
	}

	if err := d.runReducers(
		reducers,
		mCfg,
		LifecyclePreApply,
		[]string{"manifests", "terraform", ".gitignore"},
	); err != nil {
		return fmt.Errorf("error running pre-apply reducers: %w", err)
	}

	if startFrom == "" || startFrom == cluster.OperationSubPhasePreDistribution {
		// Run upgrade script if needed.
		if err := d.upgrade.Exec(d.Path, "pre-distribution"); err != nil {
			upgradeState.Phases.PreDistribution.Status = upgrade.PhaseStatusFailed

			if err := d.upgradeStore.Store(upgradeState); err != nil {
				return fmt.Errorf("error storing upgrade state: %w", err)
			}

			return fmt.Errorf("error running upgrade: %w", err)
		}

		if d.upgrade.Enabled {
			upgradeState.Phases.PreDistribution.Status = upgrade.PhaseStatusSuccess

			if err := d.upgradeStore.Store(upgradeState); err != nil {
				return fmt.Errorf("error storing upgrade state: %w", err)
			}
		}
	}

	if startFrom != cluster.OperationSubPhasePostDistribution {
		// Apply manifests.
		logrus.Info("Applying manifests...")

		if _, err := d.shellRunner.Run(path.Join(d.Path, "scripts", "apply.sh")); err != nil {
			if d.upgrade.Enabled {
				upgradeState.Phases.Distribution.Status = upgrade.PhaseStatusFailed

				if err := d.upgradeStore.Store(upgradeState); err != nil {
					return fmt.Errorf("error storing upgrade state: %w", err)
				}
			}

			return fmt.Errorf("error applying manifests: %w", err)
		}

		if d.upgrade.Enabled {
			upgradeState.Phases.Distribution.Status = upgrade.PhaseStatusSuccess

			if err := d.upgradeStore.Store(upgradeState); err != nil {
				return fmt.Errorf("error storing upgrade state: %w", err)
			}
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

	// Run upgrade script if needed.
	if err := d.upgrade.Exec(d.Path, "post-distribution"); err != nil {
		upgradeState.Phases.PostDistribution.Status = upgrade.PhaseStatusFailed

		if err := d.upgradeStore.Store(upgradeState); err != nil {
			return fmt.Errorf("error storing upgrade state: %w", err)
		}

		return fmt.Errorf("error running upgrade: %w", err)
	}

	if d.upgrade.Enabled {
		upgradeState.Phases.PostDistribution.Status = upgrade.PhaseStatusSuccess

		if err := d.upgradeStore.Store(upgradeState); err != nil {
			return fmt.Errorf("error storing upgrade state: %w", err)
		}
	}

	logrus.Info("Kubernetes Fury Distribution installed successfully")

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

		if err := d.copyFromTemplate(preTfReducersCfg); err != nil {
			return err
		}

		if _, err := d.shellRunner.Run(
			path.Join(d.Path, "scripts", fmt.Sprintf("%s.sh", lifecycle)),
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

//nolint:dupl // Remove duplicated code in the future.
func (d *Distribution) createFuryctlMerger() (*merge.Merger, error) {
	defaultsFilePath := path.Join(d.paths.DistroPath, "defaults", "onpremises-kfd-v1alpha2.yaml")

	defaultsFile, err := yamlx.FromFileV2[map[any]any](defaultsFilePath)
	if err != nil {
		return &merge.Merger{}, fmt.Errorf("%s - %w", defaultsFilePath, err)
	}

	furyctlConf, err := yamlx.FromFileV2[map[any]any](d.paths.ConfigPath)
	if err != nil {
		return &merge.Merger{}, fmt.Errorf("%s - %w", d.paths.ConfigPath, err)
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

func (d *Distribution) copyFromTemplate(cfg template.Config) error {
	outYaml, err := yamlx.MarshalV2(cfg)
	if err != nil {
		return fmt.Errorf("error marshaling template config: %w", err)
	}

	outDirPath, err := os.MkdirTemp("", "furyctl-dist-")
	if err != nil {
		return fmt.Errorf("error creating temp dir: %w", err)
	}

	confPath := filepath.Join(outDirPath, "config.yaml")

	logrus.Debugf("config path = %s", confPath)

	if err = os.WriteFile(confPath, outYaml, os.ModePerm); err != nil {
		return fmt.Errorf("error writing config file: %w", err)
	}

	templateModel, err := template.NewTemplateModel(
		path.Join(d.paths.DistroPath, "templates", cluster.OperationPhaseDistribution),
		path.Join(d.Path),
		confPath,
		outDirPath,
		d.furyctlConfPath,
		".tpl",
		false,
		d.dryRun,
	)
	if err != nil {
		return fmt.Errorf("error creating template model: %w", err)
	}

	err = templateModel.Generate()
	if err != nil {
		return fmt.Errorf("error generating from template files: %w", err)
	}

	return nil
}

func NewDistribution(
	furyctlConf public.OnpremisesKfdV1Alpha2,
	kfdManifest config.KFD,
	paths cluster.CreatorPaths,
	dryRun bool,
	upgr *upgrade.Upgrade,
) (*Distribution, error) {
	kubeDir := path.Join(paths.WorkDir, cluster.OperationPhaseDistribution)

	phase, err := cluster.NewOperationPhase(kubeDir, kfdManifest.Tools, paths.BinPath)
	if err != nil {
		return nil, fmt.Errorf("error creating distribution phase: %w", err)
	}

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
		upgradeStore: upgrade.NewStateStore(
			paths.WorkDir,
			kfdManifest.Tools.Common.Kubectl.Version,
			paths.BinPath,
		),
		upgrade: upgr,
	}, nil
}
