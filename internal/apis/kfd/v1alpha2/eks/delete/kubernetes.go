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

	"github.com/sighupio/fury-distribution/pkg/config"
	"github.com/sighupio/fury-distribution/pkg/schema/private"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/tool/awscli"
	"github.com/sighupio/furyctl/internal/tool/terraform"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
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
	*cluster.OperationPhase
	furyctlConf private.EksclusterKfdV1Alpha2
	tfRunner    *terraform.Runner
	awsRunner   *awscli.Runner
	dryRun      bool
}

func NewKubernetes(
	furyctlConf private.EksclusterKfdV1Alpha2,
	dryRun bool,
	workDir,
	binPath string,
	kfdManifest config.KFD,
) (*Kubernetes, error) {
	kubeDir := path.Join(workDir, cluster.OperationPhaseKubernetes)

	phase, err := cluster.NewOperationPhase(kubeDir, kfdManifest.Tools, binPath)
	if err != nil {
		return nil, fmt.Errorf("error creating kubernetes phase: %w", err)
	}

	return &Kubernetes{
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
		awsRunner: awscli.NewRunner(
			execx.NewStdExecutor(),
			awscli.Paths{
				Awscli:  "aws",
				WorkDir: phase.Path,
			},
		),
		dryRun: dryRun,
	}, nil
}

func (k *Kubernetes) Exec() error {
	logrus.Info("Deleting Kubernetes Fury cluster...")

	logrus.Debug("Delete: running kubernetes phase...")

	timestamp := time.Now().Unix()

	err := iox.CheckDirIsEmpty(k.OperationPhase.Path)
	if err == nil {
		logrus.Info("Kubernetes Fury cluster already deleted, skipping...")

		logrus.Debug("Kubernetes phase already executed, skipping...")

		return nil
	}

	logrus.Info("Checking connection to the VPC...")

	if err := k.checkVPCConnection(); err != nil {
		logrus.Debugf("error checking VPC connection: %v", err)

		if k.furyctlConf.Spec.Infrastructure != nil {
			if k.furyctlConf.Spec.Infrastructure.Vpn != nil {
				return fmt.Errorf("%w please check your VPN connection and try again", errKubeAPIUnreachable)
			}
		}

		return fmt.Errorf("%w please check your VPC configuration and try again", errKubeAPIUnreachable)
	}

	if err := k.tfRunner.Init(); err != nil {
		return fmt.Errorf("error running terraform init: %w", err)
	}

	if err := k.tfRunner.Plan(timestamp, "-destroy"); err != nil {
		return fmt.Errorf("error running terraform plan: %w", err)
	}

	if k.dryRun {
		return nil
	}

	logrus.Info("Deleting cloud resources, this could take a while...")

	err = k.tfRunner.Destroy()
	if err != nil {
		return fmt.Errorf("error while deleting kubernetes phase: %w", err)
	}

	return nil
}

func (k *Kubernetes) checkVPCConnection() error {
	var cidr string

	var err error

	if k.furyctlConf.Spec.Infrastructure != nil {
		cidr = string(k.furyctlConf.Spec.Infrastructure.Vpc.Network.Cidr)
	} else {
		vpcID := k.furyctlConf.Spec.Kubernetes.VpcId
		if vpcID == nil {
			return errVpcIDNotProvided
		}

		cidr, err = k.awsRunner.Ec2(
			"describe-vpcs",
			"--vpc-ids",
			string(*vpcID),
			"--query",
			"Vpcs[0].CidrBlock",
			"--output",
			"text",
		)
		if err != nil {
			return fmt.Errorf(SErrWrapWithStr, errCIDRBlockFromVpc, err)
		}
	}

	return k.queryAWSDNSServer(cidr)
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
