// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//nolint:predeclared // We want to use delete as package name.
package delete

import (
	"fmt"
	"os"
	"path"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/fury-distribution/pkg/apis/onpremises/v1alpha2/public"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/state"
	"github.com/sighupio/furyctl/internal/tool/kubectl"
	"github.com/sighupio/furyctl/internal/tool/shell"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
	"github.com/sighupio/furyctl/pkg/template"
)

type Distribution struct {
	*cluster.OperationPhase

	furyctlConf public.OnpremisesKfdV1Alpha2
	kfdManifest config.KFD
	paths       cluster.DeleterPaths
	dryRun      bool
	shellRunner *shell.Runner
	kubeRunner  *kubectl.Runner
	stateStore  state.Storer
}

func (d *Distribution) Exec() error {
	logrus.Info("Deleting Kubernetes Fury Distribution...")

	if err := d.CreateRootFolder(); err != nil {
		return fmt.Errorf("error creating distribution phase folder: %w", err)
	}

	if _, err := os.Stat(path.Join(d.OperationPhase.Path, "manifests")); os.IsNotExist(err) {
		if err := os.Mkdir(path.Join(d.OperationPhase.Path, "manifests"), iox.FullPermAccess); err != nil {
			return fmt.Errorf("error creating manifests folder: %w", err)
		}
	}

	furyctlMerger, err := d.CreateFuryctlMerger(
		d.paths.DistroPath,
		d.paths.ConfigPath,
		"kfd-v1alpha2",
		"onpremises",
	)
	if err != nil {
		return fmt.Errorf("error creating furyctl merger: %w", err)
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
		storageClassAvailable = false
	}

	mCfg.Data["checks"] = map[any]any{
		"storageClassAvailable": storageClassAvailable,
	}

	mCfg, err = d.injectStoredConfig(mCfg)
	if err != nil {
		return fmt.Errorf("error injecting stored config: %w", err)
	}

	if err := d.CopyFromTemplate(
		mCfg,
		"distribution",
		path.Join(d.paths.DistroPath, "templates", cluster.OperationPhaseDistribution),
		d.Path,
		d.paths.ConfigPath,
	); err != nil {
		return fmt.Errorf("error copying from template: %w", err)
	}

	if d.dryRun {
		logrus.Info("Kubernetes Fury Distribution deleted successfully (dry-run mode)")

		return nil
	}

	logrus.Info("Deleting kubernetes resources...")

	// Delete manifests.
	if _, err := d.shellRunner.Run(path.Join(d.Path, "scripts", "delete.sh")); err != nil {
		return fmt.Errorf("error deleting resources: %w", err)
	}

	logrus.Info("Kubernetes Fury Distribution deleted successfully")

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
	paths cluster.DeleterPaths,
	dryRun bool,
) *Distribution {
	phase := cluster.NewOperationPhase(
		path.Join(paths.WorkDir, cluster.OperationPhaseDistribution),
		kfdManifest.Tools,
		paths.BinPath,
	)

	return &Distribution{
		OperationPhase: phase,
		furyctlConf:    furyctlConf,
		kfdManifest:    kfdManifest,
		paths:          paths,
		dryRun:         dryRun,
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
		stateStore: state.NewStore(
			paths.DistroPath,
			paths.ConfigPath,
			paths.WorkDir,
			kfdManifest.Tools.Common.Kubectl.Version,
			paths.BinPath,
		),
	}
}
