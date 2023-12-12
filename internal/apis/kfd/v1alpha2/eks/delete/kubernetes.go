// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package del

import (
	"errors"
	"fmt"
	"net"
	"path"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/fury-distribution/pkg/apis/ekscluster/v1alpha2/private"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/eks/common"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/tool/awscli"
	"github.com/sighupio/furyctl/internal/tool/terraform"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	netx "github.com/sighupio/furyctl/internal/x/net"
)

const (
	SErrWrapWithStr = "%w: %s"
	// https://docs.aws.amazon.com/vpc/latest/userguide/vpc-dns.html
	awsDNSServerIPOffset = 2
)

var (
	errParsingCIDR         = errors.New("error parsing CIDR")
	errResolvingDNS        = errors.New("error resolving DNS record")
	errVpcIDNotProvided    = errors.New("vpc_id not provided")
	errCIDRBlockFromVpc    = errors.New("error getting CIDR block from VPC")
	errKubeAPIUnreachable  = errors.New("kubernetes API is not reachable")
	errAddingOffsetToIPNet = errors.New("error adding offset to ipnet")
)

type Kubernetes struct {
	*common.Kubernetes

	tfRunner  *terraform.Runner
	awsRunner *awscli.Runner
}

func NewKubernetes(
	furyctlConf private.EksclusterKfdV1Alpha2,
	dryRun bool,
	kfdManifest config.KFD,
	infraOutputsPath string,
	paths cluster.DeleterPaths,
) *Kubernetes {
	phase := cluster.NewOperationPhase(
		path.Join(paths.WorkDir, cluster.OperationPhaseKubernetes),
		kfdManifest.Tools,
		paths.BinPath,
	)

	return &Kubernetes{
		Kubernetes: &common.Kubernetes{
			OperationPhase:                     phase,
			FuryctlConf:                        furyctlConf,
			FuryctlConfPath:                    paths.ConfigPath,
			DistroPath:                         paths.DistroPath,
			KFDManifest:                        kfdManifest,
			DryRun:                             dryRun,
			InfrastructureTerraformOutputsPath: infraOutputsPath,
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
		awsRunner: awscli.NewRunner(
			execx.NewStdExecutor(),
			awscli.Paths{
				Awscli:  "aws",
				WorkDir: phase.Path,
			},
		),
	}
}

func (k *Kubernetes) Exec() error {
	logrus.Info("Deleting Kubernetes Fury cluster...")

	timestamp := time.Now().Unix()

	if err := k.Prepare(); err != nil {
		return fmt.Errorf("error preparing kubernetes phase: %w", err)
	}

	if k.FuryctlConf.Spec.Kubernetes.ApiServer.PrivateAccess &&
		!k.FuryctlConf.Spec.Kubernetes.ApiServer.PublicAccess {
		logrus.Info("Checking connection to the VPC...")

		if err := k.checkVPCConnection(); err != nil {
			logrus.Debugf("error checking VPC connection: %v", err)

			if k.FuryctlConf.Spec.Infrastructure != nil {
				if k.FuryctlConf.Spec.Infrastructure.Vpn != nil {
					return fmt.Errorf("%w please check your VPN connection and try again", errKubeAPIUnreachable)
				}
			}

			return fmt.Errorf("%w please check your VPC configuration and try again", errKubeAPIUnreachable)
		}
	}

	if err := k.tfRunner.Init(); err != nil {
		return fmt.Errorf("error running terraform init: %w", err)
	}

	if _, err := k.tfRunner.Plan(timestamp, "-destroy"); err != nil {
		return fmt.Errorf("error running terraform plan: %w", err)
	}

	if k.DryRun {
		if _, err := k.tfRunner.Plan(timestamp, "-destroy"); err != nil {
			return fmt.Errorf("error running terraform plan: %w", err)
		}

		logrus.Info("Kubernetes cluster deleted successfully (dry-run mode)")

		return nil
	}

	logrus.Warn("Deleting cloud resources, this could take a while...")

	if err := k.tfRunner.Destroy(); err != nil {
		return fmt.Errorf("error while deleting kubernetes phase: %w", err)
	}

	logrus.Info("Kubernetes cluster deleted successfully")

	return nil
}

func (k *Kubernetes) checkVPCConnection() error {
	var cidr string

	var err error

	if k.FuryctlConf.Spec.Infrastructure != nil &&
		k.FuryctlConf.Spec.Infrastructure.Vpc != nil {
		cidr = string(k.FuryctlConf.Spec.Infrastructure.Vpc.Network.Cidr)
	} else {
		vpcID := k.FuryctlConf.Spec.Kubernetes.VpcId
		if vpcID == nil {
			return errVpcIDNotProvided
		}

		cidr, err = k.awsRunner.Ec2(
			false,
			"describe-vpcs",
			"--vpc-ids",
			string(*vpcID),
			"--query",
			"Vpcs[0].CidrBlock",
			"--region",
			string(k.FuryctlConf.Spec.Region),
			"--output",
			"text",
		)
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, errCIDRBlockFromVpc, err)
		}
	}

	if k.FuryctlConf.Spec.Kubernetes.ApiServer.PrivateAccess &&
		!k.FuryctlConf.Spec.Kubernetes.ApiServer.PublicAccess &&
		k.FuryctlConf.Spec.Infrastructure.Vpn != nil {
		return k.queryAWSDNSServer(cidr)
	}

	return nil
}

func (*Kubernetes) queryAWSDNSServer(cidr string) error {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf(SErrWrapWithStr, errParsingCIDR, err)
	}

	offIPNet, err := netx.AddOffsetToIPNet(ipNet, awsDNSServerIPOffset)
	if err != nil {
		return fmt.Errorf(SErrWrapWithStr, errAddingOffsetToIPNet, err)
	}

	err = netx.DNSQuery(offIPNet.IP.String(), "google.com.")
	if err != nil {
		return fmt.Errorf(SErrWrapWithStr, errResolvingDNS, err)
	}

	return nil
}
