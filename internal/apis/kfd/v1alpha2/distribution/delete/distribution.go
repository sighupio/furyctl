// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package del

import (
	"errors"
	"fmt"
	"path"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/tool/kubectl"
	"github.com/sighupio/furyctl/internal/tool/shell"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
)

var errClusterConnect = errors.New("error connecting to cluster")

type Ingress struct {
	Name string
	Host []string
}

type Distribution struct {
	*cluster.OperationPhase
	kubeRunner  *kubectl.Runner
	shellRunner *shell.Runner
	dryRun      bool
	kubeconfig  string
}

func NewDistribution(
	dryRun bool,
	workDir,
	binPath string,
	kfdManifest config.KFD,
	kubeconfig string,
) (*Distribution, error) {
	distroDir := path.Join(workDir, cluster.OperationPhaseDistribution)

	phaseOp, err := cluster.NewOperationPhase(distroDir, kfdManifest.Tools, binPath)
	if err != nil {
		return nil, fmt.Errorf("error creating distribution phase: %w", err)
	}

	return &Distribution{
		OperationPhase: phaseOp,
		kubeRunner: kubectl.NewRunner(
			execx.NewStdExecutor(),
			kubectl.Paths{
				Kubectl:    phaseOp.KubectlPath,
				WorkDir:    path.Join(phaseOp.Path, "manifests"),
				Kubeconfig: kubeconfig,
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
	logrus.Info("Deleting Kubernetes Fury Distribution...")

	if err := iox.CheckDirIsEmpty(d.OperationPhase.Path); err == nil {
		logrus.Info("Kubernetes Fury Distribution already deleted, skipping...")

		logrus.Debug("Distribution phase already executed, skipping...")

		return nil
	}

	// Check cluster connection and requirements.
	logrus.Info("Checking that the cluster is reachable...")

	if _, err := d.kubeRunner.Version(); err != nil {
		logrus.Debugf("Got error while running cluster reachability check: %s", err)

		return errClusterConnect
	}

	if d.dryRun {
		if _, err := d.shellRunner.Run(path.Join(d.Path, "scripts", "delete.sh"), "true", d.kubeconfig); err != nil {
			return fmt.Errorf("error deleting resources: %w", err)
		}

		return nil
	}

	logrus.Info("Deleting kubernetes resources...")

	// Delete manifests.
	if _, err := d.shellRunner.Run(path.Join(d.Path, "scripts", "delete.sh"), "false", d.kubeconfig); err != nil {
		return fmt.Errorf("error deleting resources: %w", err)
	}

	return nil
}
