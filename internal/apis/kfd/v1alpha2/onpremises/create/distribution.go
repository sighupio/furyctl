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
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/merge"
	"github.com/sighupio/furyctl/internal/template"
	"github.com/sighupio/furyctl/internal/tool/kubectl"
	"github.com/sighupio/furyctl/internal/tool/shell"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

type Distribution struct {
	*cluster.OperationPhase
	furyctlConfPath string
	furyctlConf     public.OnpremisesKfdV1Alpha2
	kfdManifest     config.KFD
	paths           cluster.CreatorPaths
	dryRun          bool
	shellRunner     *shell.Runner
	kubeRunner      *kubectl.Runner
}

func (d *Distribution) Exec() error {
	logrus.Info("Installing Kubernetes Fury Distribution...")
	logrus.Debug("Create: running distribution phase...")

	if err := d.CreateFolder(); err != nil {
		return fmt.Errorf("error creating distribution phase folder: %w", err)
	}

	furyctlMerger, err := d.createFuryctlMerger()
	if err != nil {
		return err
	}

	mCfg, err := template.NewConfigWithoutData(furyctlMerger, []string{"terraform", ".gitignore", "manifests/aws"})
	if err != nil {
		return fmt.Errorf("error creating template config: %w", err)
	}

	mCfg.Data["paths"] = map[any]any{
		"kubectl":   d.OperationPhase.KubectlPath,
		"kustomize": d.OperationPhase.KustomizePath,
		"yq":        d.OperationPhase.YqPath,
	}

	// Check cluster connection and requirements.
	storageClassAvailable := true

	logrus.Info("Checking that the cluster is reachable...")

	if _, err := d.kubeRunner.Version(); err != nil {
		logrus.Debugf("Got error while running cluster reachability check: %s", err)

		return fmt.Errorf("error connecting to cluster: %w", err)
	}

	logrus.Info("Checking storage classes...")

	getStorageClassesOutput, err := d.kubeRunner.Get("", "storageclasses")
	if err != nil {
		return fmt.Errorf("error while checking storage class: %w", err)
	}

	if getStorageClassesOutput == "No resources found" {
		logrus.Warn(
			"No storage classes found in the cluster. " +
				"logging module (if enabled), dr module (if enabled) and prometheus-operated package installation will be skipped. " +
				"You need to install a StorageClass and re-run furyctl to install the missing components.",
		)
		storageClassAvailable = false
	}

	mCfg.Data["checks"] = map[any]any{
		"storageClassAvailable": storageClassAvailable,
	}

	// Generate manifests.
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
		path.Join(d.paths.DistroPath, "templates", cluster.OperationPhaseDistribution),
		path.Join(d.Path),
		confPath,
		outDirPath1,
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

	// Stop if dry run is enabled.
	if d.dryRun {
		logrus.Info("Kubernetes Fury Distribution installed successfully (dry-run mode)")

		return nil
	}

	// Apply manifests.
	logrus.Info("Applying manifests...")

	if _, err := d.shellRunner.Run(path.Join(d.Path, "scripts", "apply.sh"), "false", d.paths.Kubeconfig); err != nil {
		return fmt.Errorf("error applying manifests: %w", err)
	}

	logrus.Info("Kubernetes Fury Distribution installed successfully")

	return nil
}

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

	furyctlConfMergeModel := merge.NewDefaultModel(furyctlConf, ".spec.distribution")

	merger := merge.NewMerger(
		merge.NewDefaultModel(defaultsFile, ".data"),
		furyctlConfMergeModel,
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

func NewDistribution(
	furyctlConf public.OnpremisesKfdV1Alpha2,
	kfdManifest config.KFD,
	paths cluster.CreatorPaths,
	dryRun bool,
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
				Kubectl:    phase.KubectlPath,
				WorkDir:    path.Join(phase.Path, "manifests"),
				Kubeconfig: paths.Kubeconfig,
			},
			true,
			true,
			false,
		),
	}, nil
}
