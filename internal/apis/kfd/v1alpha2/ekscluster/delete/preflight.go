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
	"github.com/sighupio/fury-distribution/pkg/apis/ekscluster/v1alpha2/private"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/ekscluster/common"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/ekscluster/vpn"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/tool/awscli"
	"github.com/sighupio/furyctl/internal/tool/kubectl"
	"github.com/sighupio/furyctl/internal/tool/terraform"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	kubex "github.com/sighupio/furyctl/internal/x/kube"
)

var ErrClusterDoesNotExist = errors.New("cluster does not exist, nothing to delete")

type PreFlight struct {
	*common.PreFlight

	tfRunnerKube *terraform.Runner
	kubeRunner   *kubectl.Runner
}

func NewPreFlight(
	furyctlConf private.EksclusterKfdV1Alpha2,
	kfdManifest config.KFD,
	paths cluster.DeleterPaths,
	vpnAutoConnect bool,
	skipVpn bool,
	infraOutputsPath string,
) (*PreFlight, error) {
	phase := cluster.NewOperationPhase(
		path.Join(paths.WorkDir, cluster.OperationPhasePreFlight),
		kfdManifest.Tools,
		paths.BinPath,
	)

	var vpnConfig *private.SpecInfrastructureVpn
	if furyctlConf.Spec.Infrastructure != nil {
		vpnConfig = furyctlConf.Spec.Infrastructure.Vpn
	}

	vpnConnector, err := vpn.NewConnector(
		furyctlConf.Metadata.Name,
		path.Join(phase.Path, "secrets"),
		paths.BinPath,
		kfdManifest.Tools.Common.Furyagent.Version,
		vpnAutoConnect,
		skipVpn,
		vpnConfig,
	)
	if err != nil {
		return nil, fmt.Errorf("error while creating vpn connector: %w", err)
	}

	return &PreFlight{
		PreFlight: &common.PreFlight{
			OperationPhase: phase,
			FuryctlConf:    furyctlConf,
			ConfigPath:     paths.ConfigPath,
			AWSRunner: awscli.NewRunner(
				execx.NewStdExecutor(),
				awscli.Paths{
					Awscli:  "aws",
					WorkDir: paths.WorkDir,
				},
			),
			VPNConnector: vpnConnector,
			TFRunnerInfra: terraform.NewRunner(
				execx.NewStdExecutor(),
				terraform.Paths{
					WorkDir:   path.Join(phase.Path, "terraform", "infrastructure"),
					Terraform: phase.TerraformPath,
					Outputs:   infraOutputsPath,
				},
			),
			InfrastructureTerraformOutputsPath: infraOutputsPath,
		},
		tfRunnerKube: terraform.NewRunner(
			execx.NewStdExecutor(),
			terraform.Paths{
				WorkDir:   path.Join(phase.Path, "terraform", "kubernetes"),
				Terraform: phase.TerraformPath,
			},
		),
		kubeRunner: kubectl.NewRunner(
			execx.NewStdExecutor(),
			kubectl.Paths{
				Kubectl: phase.KubectlPath,
				WorkDir: phase.Path,
			},
			true,
			true,
			false,
		),
	}, nil
}

func (p *PreFlight) Exec() error {
	logrus.Info("Ensure prerequisites are in place...")

	if err := p.EnsureTerraformStateAWSS3Bucket(); err != nil {
		return fmt.Errorf("error ensuring terraform state aws s3 bucket: %w", err)
	}

	logrus.Info("Running preflight checks")

	if err := p.Prepare(); err != nil {
		return fmt.Errorf("error preparing preflight phase: %w", err)
	}

	if err := p.tfRunnerKube.Init(); err != nil {
		return fmt.Errorf("error running terraform init: %w", err)
	}

	if _, err := p.tfRunnerKube.State("show", "data.aws_eks_cluster.fury"); err != nil {
		logrus.Debug("Cluster does not exist, skipping state checks")

		logrus.Info("Preflight checks completed successfully")

		return nil //nolint:nilerr // we want to return nil here
	}

	kubeconfig := path.Join(p.Path, "secrets", "kubeconfig")

	logrus.Info("Updating kubeconfig...")

	if _, err := p.AWSRunner.Eks(
		false,
		"update-kubeconfig",
		"--name",
		p.FuryctlConf.Metadata.Name,
		"--kubeconfig",
		kubeconfig,
		"--region",
		string(p.FuryctlConf.Spec.Region),
	); err != nil {
		return fmt.Errorf("error updating kubeconfig: %w", err)
	}

	if err := kubex.SetConfigEnv(kubeconfig); err != nil {
		return fmt.Errorf("error setting kubeconfig env: %w", err)
	}

	if p.IsVPNRequired() {
		if err := p.HandleVPN(); err != nil {
			return fmt.Errorf("error handling vpn: %w", err)
		}
	}

	logrus.Info("Checking that the cluster is reachable...")

	if _, err := p.kubeRunner.Version(); err != nil {
		return fmt.Errorf("cluster is unreachable, make sure you have access to the cluster: %w", err)
	}

	if err := p.TFRunnerInfra.Init(); err != nil {
		return fmt.Errorf("error running terraform init: %w", err)
	}

	if _, err := p.TFRunnerInfra.Output(); err != nil {
		return fmt.Errorf("error getting terraform output: %w", err)
	}

	logrus.Info("Preflight checks completed successfully")

	return nil
}
