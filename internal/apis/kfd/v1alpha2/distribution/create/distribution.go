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
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/fury-distribution/pkg/apis/kfddistribution/v1alpha2/public"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/merge"
	"github.com/sighupio/furyctl/internal/template"
	"github.com/sighupio/furyctl/internal/tool/kubectl"
	"github.com/sighupio/furyctl/internal/tool/shell"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

var (
	errClusterConnect = errors.New("error connecting to cluster")
	errNoStorageClass = errors.New("at least one storage class is required")
	errNodesNotReady  = errors.New("all nodes should be Ready")
)

type Distribution struct {
	*cluster.OperationPhase
	furyctlConfPath string
	furyctlConf     public.KfddistributionKfdV1Alpha2
	distroPath      string
	kubeRunner      *kubectl.Runner
	dryRun          bool
	shellRunner     *shell.Runner
	kubeconfig      string
}

func NewDistribution(
	paths cluster.CreatorPaths,
	furyctlConf public.KfddistributionKfdV1Alpha2,
	kfdManifest config.KFD,
	dryRun bool,
	kubeconfig string,
) (*Distribution, error) {
	distroDir := path.Join(paths.WorkDir, cluster.OperationPhaseDistribution)

	phaseOp, err := cluster.NewOperationPhase(distroDir, kfdManifest.Tools, paths.BinPath)
	if err != nil {
		return nil, fmt.Errorf("error creating distribution phase: %w", err)
	}

	return &Distribution{
		OperationPhase:  phaseOp,
		furyctlConf:     furyctlConf,
		distroPath:      paths.DistroPath,
		furyctlConfPath: paths.ConfigPath,
		kubeRunner: kubectl.NewRunner(
			execx.NewStdExecutor(),
			kubectl.Paths{
				Kubectl:    phaseOp.KubectlPath,
				WorkDir:    path.Join(phaseOp.Path, "manifests"),
				Kubeconfig: paths.Kubeconfig,
			},
			true,
			true,
			false,
		),
		shellRunner: shell.NewRunner(
			execx.NewStdExecutor(),
			shell.Paths{
				Shell:   "sh",
				WorkDir: path.Join(phaseOp.Path, "manifests"),
			},
		),
		dryRun:     dryRun,
		kubeconfig: kubeconfig,
	}, nil
}

func (d *Distribution) Exec() error {
	logrus.Info("Installing Kubernetes Fury Distribution...")

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
		path.Join(d.distroPath, "templates", cluster.OperationPhaseDistribution),
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
		if _, err := d.shellRunner.Run(path.Join(d.Path, "scripts", "apply.sh"), "true", d.kubeconfig); err != nil {
			return fmt.Errorf("error applying resources: %w", err)
		}

		return nil
	}

	if d.furyctlConf.Spec.Distribution.Modules.Networking.Type == "none" {
		logrus.Info("Checking if all nodes are ready...")

		getNotReadyNodesOutput, err := d.kubeRunner.Get(
			"",
			"nodes",
			"--output",
			"jsonpath=\"{range .items[*]}{.spec.taints[?(@.key==\"node.kubernetes.io/not-ready\")]}{end}\"",
		)
		if err != nil {
			return fmt.Errorf("error while checking nodes: %w", err)
		}

		if getNotReadyNodesOutput != "\"\"" {
			return errNodesNotReady
		}
	}

	// Apply manifests.
	logrus.Info("Applying manifests...")

	if _, err := d.shellRunner.Run(path.Join(d.Path, "scripts", "apply.sh"), "false", d.kubeconfig); err != nil {
		return fmt.Errorf("error applying manifests: %w", err)
	}

	return nil
}

func (d *Distribution) Stop() error {
	errCh := make(chan error)
	doneCh := make(chan bool)

	var wg sync.WaitGroup

	//nolint:gomnd // ignore magic number linters
	wg.Add(2)

	go func() {
		logrus.Debug("Stopping shell...")

		if err := d.shellRunner.Stop(); err != nil {
			errCh <- fmt.Errorf("error stopping shell: %w", err)
		}

		wg.Done()
	}()

	go func() {
		logrus.Debug("Stopping kubectl...")

		if err := d.kubeRunner.Stop(); err != nil {
			errCh <- fmt.Errorf("error stopping kubectl: %w", err)
		}

		wg.Done()
	}()

	go func() {
		wg.Wait()
		close(doneCh)
	}()

	select {
	case <-doneCh:

	case err := <-errCh:
		close(errCh)

		return err
	}

	return nil
}

func (d *Distribution) createFuryctlMerger() (*merge.Merger, error) {
	defaultsFilePath := path.Join(d.distroPath, "defaults", "kfddistribution-kfd-v1alpha2.yaml")

	defaultsFile, err := yamlx.FromFileV2[map[any]any](defaultsFilePath)
	if err != nil {
		return &merge.Merger{}, fmt.Errorf("%s - %w", defaultsFilePath, err)
	}

	furyctlConf, err := yamlx.FromFileV2[map[any]any](d.furyctlConfPath)
	if err != nil {
		return &merge.Merger{}, fmt.Errorf("%s - %w", d.furyctlConfPath, err)
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
