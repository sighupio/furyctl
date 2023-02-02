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
	"github.com/sighupio/fury-distribution/pkg/schema"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/tool/terraform"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
	osx "github.com/sighupio/furyctl/internal/x/os"
)

type Infrastructure struct {
	*cluster.OperationPhase
	furyctlConf schema.EksclusterKfdV1Alpha2
	tfRunner    *terraform.Runner
	dryRun      bool
}

func NewInfrastructure(
	furyctlConf schema.EksclusterKfdV1Alpha2,
	dryRun bool,
	workDir,
	binPath string,
	kfdManifest config.KFD,
) (*Infrastructure, error) {
	infraDir := path.Join(workDir, cluster.OperationPhaseInfrastructure)

	phase, err := cluster.NewOperationPhase(infraDir, kfdManifest.Tools, binPath)
	if err != nil {
		return nil, fmt.Errorf("error creating infrastructure phase: %w", err)
	}

	return &Infrastructure{
		OperationPhase: phase,
		furyctlConf:    furyctlConf,
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
	logrus.Info("Deleting infrastructure...")

	logrus.Debug("Delete: running infrastructure phase...")

	timestamp := time.Now().Unix()

	err := iox.CheckDirIsEmpty(i.OperationPhase.Path)
	if err == nil {
		logrus.Info("Infrastructure already deleted, skipping...")

		logrus.Debug("Infrastructure phase already executed, skipping...")

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

	if i.isVpnConfigured() {
		killMsg := "killall openvpn"

		isRoot, err := osx.IsRoot()
		if err != nil {
			return fmt.Errorf("error while checking if user is root: %w", err)
		}

		if !isRoot {
			killMsg = fmt.Sprintf("sudo %s", killMsg)
		}

		logrus.Warnf("Please, remember to kill the OpenVPN process, "+
			"you can do it with the following command: '%s'", killMsg)
	}

	return nil
}

func (i *Infrastructure) isVpnConfigured() bool {
	if i.furyctlConf.Spec.Infrastructure == nil {
		return false
	}

	vpn := i.furyctlConf.Spec.Infrastructure.Vpc.Vpn
	if vpn == nil {
		return false
	}

	instances := i.furyctlConf.Spec.Infrastructure.Vpc.Vpn.Instances
	if instances == nil {
		return true
	}

	return *instances > 0
}
