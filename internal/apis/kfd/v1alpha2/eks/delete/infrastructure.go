// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package del

import (
	"fmt"
	"path"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/config"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/tool/terraform"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
)

type Infrastructure struct {
	*cluster.OperationPhase
	tfRunner *terraform.Runner
	dryRun   bool
}

func NewInfrastructure(dryRun bool, workDir, binPath string, kfdManifest config.KFD) (*Infrastructure, error) {
	infraDir := path.Join(workDir, cluster.OperationPhaseInfrastructure)

	phase, err := cluster.NewOperationPhase(infraDir, kfdManifest.Tools, binPath)
	if err != nil {
		return nil, fmt.Errorf("error creating infrastructure phase: %w", err)
	}

	return &Infrastructure{
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

func (i *Infrastructure) Exec() error {
	logrus.Info("Deleting infrastructure phase...")

	timestamp := time.Now().Unix()

	err := iox.CheckDirIsEmpty(i.OperationPhase.Path)
	if err == nil {
		logrus.Infof("infrastructure phase already executed, skipping...")

		return nil
	}

	if err := i.tfRunner.Plan(timestamp, "-destroy"); err != nil {
		return fmt.Errorf("error running terraform plan: %w", err)
	}

	if i.dryRun {
		return nil
	}

	err = i.tfRunner.Destroy()
	if err != nil {
		return fmt.Errorf("error while deleting infrastructure: %w", err)
	}

	logrus.Warnf("Please, remember to kill the OpenVPN process if" +
		" you have chosen to create it in the infrastructure phase")

	return nil
}
