// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package del

import (
	"errors"
	"fmt"
	"path"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/fury-distribution/pkg/apis/ekscluster/v1alpha2/private"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/ekscluster/common"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/kubernetes"
	"github.com/sighupio/furyctl/internal/state"
	"github.com/sighupio/furyctl/internal/tool/awscli"
	"github.com/sighupio/furyctl/internal/tool/shell"
	"github.com/sighupio/furyctl/internal/tool/terraform"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

var errClusterConnect = errors.New("error connecting to cluster")

type Ingress struct {
	Name string
	Host []string
}

type Distribution struct {
	*common.Distribution

	awsRunner   *awscli.Runner
	shellRunner *shell.Runner
	kubeClient  *kubernetes.Client
	dryRun      bool
	paths       cluster.DeleterPaths
}

func NewDistribution(
	dryRun bool,
	kfdManifest config.KFD,
	infraOutputsPath string,
	paths cluster.DeleterPaths,
	furyctlConf private.EksclusterKfdV1Alpha2,
) *Distribution {
	phase := cluster.NewOperationPhase(
		path.Join(paths.WorkDir, cluster.OperationPhaseDistribution),
		kfdManifest.Tools,
		paths.BinPath,
	)

	return &Distribution{
		Distribution: &common.Distribution{
			OperationPhase:                     phase,
			DryRun:                             dryRun,
			DistroPath:                         paths.DistroPath,
			ConfigPath:                         paths.ConfigPath,
			InfrastructureTerraformOutputsPath: infraOutputsPath,
			FuryctlConf:                        furyctlConf,
			StateStore: state.NewStore(
				paths.DistroPath,
				paths.ConfigPath,
				paths.WorkDir,
				kfdManifest.Tools.Common.Kubectl.Version,
				paths.BinPath,
			),
			TFRunner: terraform.NewRunner(
				execx.NewStdExecutor(),
				terraform.Paths{
					Logs:      phase.TerraformLogsPath,
					Outputs:   phase.TerraformOutputsPath,
					WorkDir:   path.Join(phase.Path, "terraform"),
					Plan:      phase.TerraformPlanPath,
					Terraform: phase.TerraformPath,
				},
			),
		},
		awsRunner: awscli.NewRunner(
			execx.NewStdExecutor(),
			awscli.Paths{
				Awscli:  "aws",
				WorkDir: phase.Path,
			},
		),
		kubeClient: kubernetes.NewClient(
			phase.KubectlPath,
			path.Join(phase.Path, "manifests"),
			true,
			true,
			false,
			execx.NewStdExecutor(),
		),
		shellRunner: shell.NewRunner(
			execx.NewStdExecutor(),
			shell.Paths{
				Shell:   "sh",
				WorkDir: path.Join(phase.Path, "manifests"),
			},
		),
		dryRun: dryRun,
		paths:  paths,
	}
}

func (d *Distribution) Exec() error {
	logrus.Info("Deleting Kubernetes Fury Distribution...")

	furyctlMerger, preTfMerger, _, err := d.PreparePreTerraform()
	if err != nil {
		return fmt.Errorf("error preparing distribution phase (pre terraform): %w", err)
	}

	if err := d.TFRunner.Init(); err != nil {
		return fmt.Errorf("error running terraform init: %w", err)
	}

	if _, err := d.TFRunner.Output(); err != nil {
		return fmt.Errorf("error running terraform output: %w", err)
	}

	_, err = d.PreparePostTerraform(
		furyctlMerger,
		preTfMerger,
	)
	if err != nil {
		return fmt.Errorf("error preparing distribution phase (post terraform): %w", err)
	}

	logrus.Info("Checking cluster connectivity...")

	if _, err := d.kubeClient.ToolVersion(); err != nil {
		return fmt.Errorf("%w: %w", errClusterConnect, err)
	}

	if d.dryRun {
		timestamp := time.Now().Unix()

		if _, err := d.TFRunner.Plan(timestamp, "-destroy"); err != nil {
			return fmt.Errorf("error running terraform plan: %w", err)
		}

		logrus.Info("The following resources, regardless of the built manifests, are going to be deleted:")

		if _, err := d.kubeClient.ListNamespaceResources("ingress", "all"); err != nil {
			logrus.Errorf("error while getting list of ingress resources: %v", err)
		}

		if _, err := d.kubeClient.ListNamespaceResources("prometheus", "monitoring"); err != nil {
			logrus.Errorf("error while getting list of prometheus resources: %v", err)
		}

		if _, err := d.kubeClient.ListNamespaceResources("persistentvolumeclaim", "monitoring"); err != nil {
			logrus.Errorf("error while getting list of persistentvolumeclaim resources: %v", err)
		}

		if _, err := d.kubeClient.ListNamespaceResources("persistentvolumeclaim", "logging"); err != nil {
			logrus.Errorf("error while getting list of persistentvolumeclaim resources: %v", err)
		}

		if _, err := d.kubeClient.ListNamespaceResources("statefulset", "logging"); err != nil {
			logrus.Errorf("error while getting list of statefulset resources: %v", err)
		}

		if _, err := d.kubeClient.ListNamespaceResources("logging", "logging"); err != nil {
			logrus.Errorf("error while getting list of logging resources: %v", err)
		}

		if _, err := d.kubeClient.ListNamespaceResources("service", "ingress-nginx"); err != nil {
			logrus.Errorf("error while getting list of service resources: %v", err)
		}

		logrus.Info("Kubernetes Fury Distribution deleted successfully (dry-run mode)")

		return nil
	}

	logrus.Info("Deleting kubernetes resources...")

	if _, err := d.shellRunner.Run(path.Join(d.Path, "scripts", "delete.sh")); err != nil {
		return fmt.Errorf("error deleting resources: %w", err)
	}

	logrus.Info("Deleting infra resources...")

	if err := d.TFRunner.Destroy(); err != nil {
		return fmt.Errorf("error while deleting infra resources: %w", err)
	}

	logrus.Info("Kubernetes Fury Distribution deleted successfully")

	return nil
}
