// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//nolint:dupl // better readability
package del

import (
	"fmt"
	"path"

	"github.com/sirupsen/logrus"

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

func NewInfrastructure(dryRun bool) (*Infrastructure, error) {
	phase, err := cluster.NewOperationPhase(".infrastructure")
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
	logrus.Info("Deleting infrastructure phase")

	err := iox.CheckDirIsEmpty(i.OperationPhase.Path)
	if err == nil {
		logrus.Infof("infrastructure phase already executed, skipping")

		return nil
	}

	err = i.tfRunner.Destroy()
	if err != nil {
		return fmt.Errorf("error running terraform destroy: %w", err)
	}

	return nil
}
