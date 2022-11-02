// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package del

import (
	"fmt"
	"path"

	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/tool/terraform"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

type Kubernetes struct {
	tfRunner *terraform.Runner
}

func NewKubernetes() (*Kubernetes, error) {
	phase, err := cluster.NewOperationPhase(".kubernetes")
	if err != nil {
		return nil, fmt.Errorf("error creating kubernetes phase: %w", err)
	}

	return &Kubernetes{
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
	}, nil
}

func (*Kubernetes) Exec() error {
	return nil
}
