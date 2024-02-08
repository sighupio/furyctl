// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package create

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/fury-distribution/pkg/apis/ekscluster/v1alpha2/private"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/ekscluster/common"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/parser"
	"github.com/sighupio/furyctl/internal/tool/terraform"
	"github.com/sighupio/furyctl/internal/upgrade"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
	"github.com/sighupio/furyctl/internal/x/slices"
)

var ErrAbortedByUser = errors.New("aborted by user")

type Infrastructure struct {
	*common.Infrastructure

	kfdManifest config.KFD
	tfRunner    *terraform.Runner
	dryRun      bool
	upgrade     *upgrade.Upgrade
	paths       cluster.CreatorPaths
}

func NewInfrastructure(
	furyctlConf private.EksclusterKfdV1Alpha2,
	kfdManifest config.KFD,
	paths cluster.CreatorPaths,
	dryRun bool,
	upgr *upgrade.Upgrade,
) *Infrastructure {
	phase := cluster.NewOperationPhase(
		path.Join(paths.WorkDir, cluster.OperationPhaseInfrastructure),
		kfdManifest.Tools,
		paths.BinPath,
	)

	executor := execx.NewStdExecutor()

	return &Infrastructure{
		Infrastructure: &common.Infrastructure{
			OperationPhase: phase,
			FuryctlConf:    furyctlConf,
			ConfigPath:     paths.ConfigPath,
		},
		kfdManifest: kfdManifest,
		tfRunner: terraform.NewRunner(
			executor,
			terraform.Paths{
				Logs:      phase.TerraformLogsPath,
				Outputs:   phase.TerraformOutputsPath,
				WorkDir:   path.Join(phase.Path, "terraform"),
				Plan:      phase.TerraformPlanPath,
				Terraform: phase.TerraformPath,
			},
		),
		dryRun:  dryRun,
		upgrade: upgr,
		paths:   paths,
	}
}

func (i *Infrastructure) Self() *cluster.OperationPhase {
	return i.OperationPhase
}

func (i *Infrastructure) Exec(startFrom string, upgradeState *upgrade.State) error {
	logrus.Info("Creating infrastructure...")

	timestamp := time.Now().Unix()

	if err := i.Prepare(); err != nil {
		return fmt.Errorf("error preparing infrastructure phase: %w", err)
	}

	if err := i.tfRunner.Init(); err != nil {
		return fmt.Errorf("error running terraform init: %w", err)
	}

	if err := i.preInfrastructure(startFrom, upgradeState); err != nil {
		return fmt.Errorf("error running pre-infrastructure phase: %w", err)
	}

	if err := i.coreInfrastructure(startFrom, upgradeState, timestamp); err != nil {
		return fmt.Errorf("error running core infrastructure phase: %w", err)
	}

	if i.dryRun {
		return nil
	}

	if err := i.postInfrastructure(upgradeState); err != nil {
		return fmt.Errorf("error running post-infrastructure phase: %w", err)
	}

	logrus.Info("Infrastructure created successfully")

	return nil
}

func (i *Infrastructure) preInfrastructure(
	startFrom string,
	upgradeState *upgrade.State,
) error {
	if !i.dryRun && (startFrom == "" || startFrom == cluster.OperationSubPhasePreInfrastructure) {
		if err := i.upgrade.Exec(i.Path, "pre-infrastructure"); err != nil {
			upgradeState.Phases.PreInfrastructure.Status = upgrade.PhaseStatusFailed

			return fmt.Errorf("error running upgrade: %w", err)
		}

		if i.upgrade.Enabled {
			upgradeState.Phases.PreInfrastructure.Status = upgrade.PhaseStatusSuccess
		}
	}

	return nil
}

func (i *Infrastructure) coreInfrastructure(
	startFrom string,
	upgradeState *upgrade.State,
	timestamp int64,
) error {
	if startFrom != cluster.OperationSubPhasePostInfrastructure {
		plan, err := i.tfRunner.Plan(timestamp)
		if err != nil {
			return fmt.Errorf("error running terraform plan: %w", err)
		}

		if i.dryRun {
			return nil
		}

		tfParser := parser.NewTfPlanParser(string(plan))

		parsedPlan := tfParser.Parse()

		criticalResources := slices.Intersection(i.getCriticalTFResourceTypes(), parsedPlan.Destroy)

		if len(criticalResources) > 0 {
			logrus.Warnf("Deletion of the following critical resources has been detected: %s. See the logs for more details.",
				strings.Join(criticalResources, ", "))
			logrus.Warn("Do you want to proceed? write 'yes' to continue or anything else to abort: ")

			prompter := iox.NewPrompter(bufio.NewReader(os.Stdin))

			prompt, err := prompter.Ask("yes")
			if err != nil {
				return fmt.Errorf("error reading user input: %w", err)
			}

			if !prompt {
				return ErrAbortedByUser
			}
		}

		logrus.Warn("Creating cloud resources, this could take a while...")

		if err := i.tfRunner.Apply(timestamp); err != nil {
			if i.upgrade.Enabled {
				upgradeState.Phases.Infrastructure.Status = upgrade.PhaseStatusFailed
			}

			return fmt.Errorf("cannot create cloud resources: %w", err)
		}

		if i.upgrade.Enabled {
			upgradeState.Phases.Infrastructure.Status = upgrade.PhaseStatusSuccess
		}

		if _, err := i.tfRunner.Output(); err != nil {
			return fmt.Errorf("error getting terraform output: %w", err)
		}
	}

	return nil
}

func (*Infrastructure) getCriticalTFResourceTypes() []string {
	return []string{"aws_vpc", "aws_subnet"}
}

func (i *Infrastructure) postInfrastructure(
	upgradeState *upgrade.State,
) error {
	if err := i.upgrade.Exec(i.Path, "post-infrastructure"); err != nil {
		upgradeState.Phases.PostInfrastructure.Status = upgrade.PhaseStatusFailed

		return fmt.Errorf("error running upgrade: %w", err)
	}

	if i.upgrade.Enabled {
		upgradeState.Phases.PostInfrastructure.Status = upgrade.PhaseStatusSuccess
	}

	return nil
}

func (i *Infrastructure) Stop() error {
	logrus.Debug("Stopping terraform...")

	if err := i.tfRunner.Stop(); err != nil {
		return fmt.Errorf("error stopping terraform: %w", err)
	}

	return nil
}
