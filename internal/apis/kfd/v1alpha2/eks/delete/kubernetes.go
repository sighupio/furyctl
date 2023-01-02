// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//nolint:dupl // better readability
package del

import (
	"fmt"
	"path"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/config"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/tool/terraform"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
)

type Kubernetes struct {
	*cluster.OperationPhase
	tfRunner *terraform.Runner
	dryRun   bool
}

func NewKubernetes(dryRun bool, workDir, binPath string, kfdManifest config.KFD) (*Kubernetes, error) {
	kubeDir := path.Join(workDir, "kubernetes")

	phase, err := cluster.NewOperationPhase(kubeDir, kfdManifest.Tools, binPath)
	if err != nil {
		return nil, fmt.Errorf("error creating kubernetes phase: %w", err)
	}

	return &Kubernetes{
		OperationPhase: phase,
		tfRunner: terraform.NewRunner(
			execx.NewStdExecutor(),
			terraform.Paths{
				Logs:      phase.LogsPath,
				Outputs:   phase.OutputsPath,
				WorkDir:   path.Join(phase.Path, "terraform"),
				Plan:      phase.PlanPath,
				Terraform: phase.TerraformPath,
			},
		),
		dryRun: dryRun,
	}, nil
}

func (k *Kubernetes) Exec() error {
	logrus.Info("Deleting kubernetes phase...")

	err := iox.CheckDirIsEmpty(k.OperationPhase.Path)
	if err == nil {
		logrus.Infof("kubernetes phase already executed, skipping...")

		return nil
	}

	err = k.tfRunner.Destroy()
	if err != nil {
		return fmt.Errorf("error while deleting kubernetes phase: %w", err)
	}

	return nil
}
