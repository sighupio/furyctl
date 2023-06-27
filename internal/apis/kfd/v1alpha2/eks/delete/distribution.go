// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package del

import (
	"errors"
	"fmt"
	"path"
	"regexp"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/kubernetes"
	"github.com/sighupio/furyctl/internal/tool/awscli"
	"github.com/sighupio/furyctl/internal/tool/shell"
	"github.com/sighupio/furyctl/internal/tool/terraform"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
)

const (
	ingressAfterDeleteDelay         = 4
	checkPendingResourcesDelay      = 20
	checkPendingResourcesMaxRetries = 5
)

var (
	errCheckPendingResources = errors.New("error while checking pending resources")
	errPendingResources      = errors.New("pending resources: ")
	errClusterConnect        = errors.New("error connecting to cluster")
	hostedZoneRegex          = regexp.MustCompile(`/hostedzone/(\S+)\t(\S+)\.`)
	recordSetsRegex          = regexp.MustCompile(`(\S+)\.`)
)

type Ingress struct {
	Name string
	Host []string
}

type Distribution struct {
	*cluster.OperationPhase
	tfRunner    *terraform.Runner
	awsRunner   *awscli.Runner
	shellRunner *shell.Runner
	kubeClient  *kubernetes.Client
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

	phase, err := cluster.NewOperationPhase(distroDir, kfdManifest.Tools, binPath)
	if err != nil {
		return nil, fmt.Errorf("error creating distribution phase: %w", err)
	}

	return &Distribution{
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
			kubeconfig,
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
		kubeconfig: kubeconfig,
		dryRun:     dryRun,
	}, nil
}

func (d *Distribution) Exec() error {
	logrus.Info("Deleting Kubernetes Fury Distribution...")

	logrus.Debug("Delete: running distribution phase...")

	if err := iox.CheckDirIsEmpty(d.OperationPhase.Path); err == nil {
		logrus.Info("Kubernetes Fury Distribution already deleted, skipping...")

		logrus.Debug("Distribution phase already executed, skipping...")

		return nil
	}

	logrus.Info("Checking cluster connectivity...")

	if _, err := d.kubeClient.ToolVersion(); err != nil {
		return errClusterConnect
	}

	if d.dryRun {
		timestamp := time.Now().Unix()

		if err := d.tfRunner.Init(); err != nil {
			return fmt.Errorf("error running terraform init: %w", err)
		}

		if _, err := d.tfRunner.Plan(timestamp, "-destroy"); err != nil {
			return fmt.Errorf("error running terraform plan: %w", err)
		}

		if _, err := d.shellRunner.Run(path.Join(d.Path, "scripts", "delete.sh"), "true", d.kubeconfig); err != nil {
			return fmt.Errorf("error deleting resources: %w", err)
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

		return nil
	}

	logrus.Info("Deleting kubernetes resources...")

	if _, err := d.shellRunner.Run(path.Join(d.Path, "scripts", "delete.sh"), "false", d.kubeconfig); err != nil {
		return fmt.Errorf("error deleting resources: %w", err)
	}

	logrus.Info("Deleting infra resources...")

	if err := d.tfRunner.Destroy(); err != nil {
		return fmt.Errorf("error while deleting infra resources: %w", err)
	}

	return nil
}
