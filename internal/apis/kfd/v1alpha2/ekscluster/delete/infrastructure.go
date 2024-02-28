// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package del

import (
	"fmt"
	"path"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/fury-distribution/pkg/apis/ekscluster/v1alpha2/private"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/ekscluster/common"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/tool/terraform"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	osx "github.com/sighupio/furyctl/internal/x/os"
)

type Infrastructure struct {
	*common.Infrastructure

	tfRunner *terraform.Runner
	dryRun   bool
}

func NewInfrastructure(
	furyctlConf private.EksclusterKfdV1Alpha2,
	dryRun bool,
	kfdManifest config.KFD,
	paths cluster.DeleterPaths,
) *Infrastructure {
	phase := cluster.NewOperationPhase(
		path.Join(paths.WorkDir, cluster.OperationPhaseInfrastructure),
		kfdManifest.Tools,
		paths.BinPath,
	)

	return &Infrastructure{
		Infrastructure: &common.Infrastructure{
			OperationPhase: phase,
			FuryctlConf:    furyctlConf,
			ConfigPath:     paths.ConfigPath,
			DistroPath:     paths.DistroPath,
		},
		tfRunner: terraform.NewRunner(
			execx.NewStdExecutor(),
			terraform.Paths{
				Logs:      phase.TerraformLogsPath,
				Outputs:   phase.TerraformOutputsPath,
				WorkDir:   path.Join(phase.Path, "terraform"),
				Plan:      phase.TerraformPlanPath,
				Terraform: phase.TerraformPath,
			},
		),
		dryRun: dryRun,
	}
}

func (i *Infrastructure) Exec() error {
	logrus.Info("Deleting infrastructure...")

	if err := i.Prepare(); err != nil {
		return fmt.Errorf("error preparing infrastructure phase: %w", err)
	}

	if err := i.tfRunner.Init(); err != nil {
		return fmt.Errorf("error running terraform init: %w", err)
	}

	if i.dryRun {
		if _, err := i.tfRunner.Plan(time.Now().Unix(), "-destroy"); err != nil {
			return fmt.Errorf("error running terraform plan: %w", err)
		}

		logrus.Info("Infrastructure deleted successfully (dry-run mode)")

		return nil
	}

	if err := i.tfRunner.Destroy(); err != nil {
		return fmt.Errorf("error while deleting infrastructure: %w", err)
	}

	if i.isVpnConfigured() &&
		i.FuryctlConf.Spec.Kubernetes.ApiServer.PrivateAccess &&
		!i.FuryctlConf.Spec.Kubernetes.ApiServer.PublicAccess {
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

	logrus.Info("Infrastructure deleted successfully")

	return nil
}

func (i *Infrastructure) isVpnConfigured() bool {
	if i.FuryctlConf.Spec.Infrastructure == nil {
		return false
	}

	vpn := i.FuryctlConf.Spec.Infrastructure.Vpn
	if vpn == nil {
		return false
	}

	instances := i.FuryctlConf.Spec.Infrastructure.Vpn.Instances
	if instances == nil {
		return true
	}

	return *instances > 0
}
