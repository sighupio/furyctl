// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package create

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/fury-distribution/pkg/apis/ekscluster/v1alpha2/private"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/ekscluster/common"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/parser"
	"github.com/sighupio/furyctl/internal/tool/awscli"
	"github.com/sighupio/furyctl/internal/tool/terraform"
	"github.com/sighupio/furyctl/internal/upgrade"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
	kubex "github.com/sighupio/furyctl/internal/x/kube"
	netx "github.com/sighupio/furyctl/internal/x/net"
	"github.com/sighupio/furyctl/internal/x/slices"
)

var (
	errMissingKubeconfig   = errors.New("kubeconfig not found in infrastructure phase's logs")
	errWrongKubeconfig     = errors.New("kubeconfig cannot be parsed from infrastructure phase's logs")
	errParsingCIDR         = errors.New("error parsing CIDR")
	errResolvingDNS        = errors.New("error resolving DNS record")
	errVpcIDNotProvided    = errors.New("vpcId not provided")
	errCIDRBlockFromVpc    = errors.New("error getting CIDR block from VPC")
	errKubeAPIUnreachable  = errors.New("kubernetes API is not reachable")
	errAddingOffsetToIPNet = errors.New("error adding offset to ipnet")
)

const (
	// https://docs.aws.amazon.com/vpc/latest/userguide/vpc-dns.html
	awsDNSServerIPOffset = 2
)

type Kubernetes struct {
	*common.Kubernetes

	tfRunner  *terraform.Runner
	awsRunner *awscli.Runner
	upgrade   *upgrade.Upgrade
	paths     cluster.CreatorPaths
}

func NewKubernetes(
	furyctlConf private.EksclusterKfdV1Alpha2,
	kfdManifest config.KFD,
	infraOutputsPath string,
	paths cluster.CreatorPaths,
	dryRun bool,
	upgr *upgrade.Upgrade,
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
		upgrade: upgr,
		paths:   paths,
	}
}

func (k *Kubernetes) Self() *cluster.OperationPhase {
	return k.OperationPhase
}

func (k *Kubernetes) Exec(startFrom string, upgradeState *upgrade.State) error {
	timestamp := time.Now().Unix()

	logrus.Info("Creating Kubernetes Fury cluster...")

	if err := k.Prepare(); err != nil {
		return fmt.Errorf("error preparing kubernetes phase: %w", err)
	}

	if err := k.tfRunner.Init(); err != nil {
		return fmt.Errorf("error running terraform init: %w", err)
	}

	if err := k.preKubernetes(startFrom, upgradeState); err != nil {
		return fmt.Errorf("error running pre-kubernetes phase: %w", err)
	}

	if err := k.coreKubernetes(startFrom, upgradeState, timestamp); err != nil {
		return fmt.Errorf("error running core kubernetes phase: %w", err)
	}

	if k.DryRun {
		logrus.Info("Kubernetes cluster created successfully (dry-run mode)")

		return nil
	}

	if err := k.postKubernetes(upgradeState); err != nil {
		return fmt.Errorf("error running post-kubernetes phase: %w", err)
	}

	logrus.Info("Kubernetes cluster created successfully")

	return nil
}

func (k *Kubernetes) preKubernetes(
	startFrom string,
	upgradeState *upgrade.State,
) error {
	if !k.DryRun && (startFrom == "" || startFrom == cluster.OperationSubPhasePreKubernetes) {
		if err := k.upgrade.Exec(k.Path, "pre-kubernetes"); err != nil {
			upgradeState.Phases.PreKubernetes.Status = upgrade.PhaseStatusFailed

			return fmt.Errorf("error running upgrade: %w", err)
		}

		if k.upgrade.Enabled {
			upgradeState.Phases.PreKubernetes.Status = upgrade.PhaseStatusSuccess
		}
	}

	return nil
}

func (k *Kubernetes) coreKubernetes(
	startFrom string,
	upgradeState *upgrade.State,
	timestamp int64,
) error {
	if startFrom != cluster.OperationSubPhasePostKubernetes {
		plan, err := k.tfRunner.Plan(timestamp)
		if err != nil {
			return fmt.Errorf("error running terraform plan: %w", err)
		}

		if k.DryRun {
			return nil
		}

		tfParser := parser.NewTfPlanParser(string(plan))

		parsedPlan := tfParser.Parse()

		criticalResources := slices.Intersection(k.getCriticalTFResourceTypes(), parsedPlan.Destroy)

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

		logrus.Warn("Creating cloud resources, this could take a while...")

		if err := k.tfRunner.Apply(timestamp); err != nil {
			if k.upgrade.Enabled {
				upgradeState.Phases.Kubernetes.Status = upgrade.PhaseStatusFailed
			}

			return fmt.Errorf("cannot create cloud resources: %w", err)
		}

		if k.upgrade.Enabled {
			upgradeState.Phases.Kubernetes.Status = upgrade.PhaseStatusSuccess
		}

		out, err := k.tfRunner.Output()
		if err != nil {
			return fmt.Errorf("error getting terraform output: %w", err)
		}

		if out["kubeconfig"] == nil {
			return errMissingKubeconfig
		}

		kubeString, ok := out["kubeconfig"].Value.(string)
		if !ok {
			return errWrongKubeconfig
		}

		p, err := kubex.CreateConfig([]byte(kubeString), k.TerraformSecretsPath)
		if err != nil {
			return fmt.Errorf("error creating kubeconfig: %w", err)
		}

		if err := kubex.SetConfigEnv(p); err != nil {
			return fmt.Errorf("error setting kubeconfig env: %w", err)
		}

		if err := kubex.CopyToWorkDir(p, "kubeconfig"); err != nil {
			return fmt.Errorf("error copying kubeconfig: %w", err)
		}
	}

	return nil
}

func (*Kubernetes) getCriticalTFResourceTypes() []string {
	return []string{"aws_eks_cluster"}
}

func (k *Kubernetes) postKubernetes(
	upgradeState *upgrade.State,
) error {
	if err := k.upgrade.Exec(k.Path, "post-kubernetes"); err != nil {
		upgradeState.Phases.PostKubernetes.Status = upgrade.PhaseStatusFailed

		return fmt.Errorf("error running upgrade: %w", err)
	}

	if k.upgrade.Enabled {
		upgradeState.Phases.PostKubernetes.Status = upgrade.PhaseStatusSuccess
	}

	return nil
}

func (k *Kubernetes) SetUpgrade(upgradeEnabled bool) {
	k.upgrade.Enabled = upgradeEnabled
}

func (k *Kubernetes) Stop() error {
	errCh := make(chan error)
	doneCh := make(chan bool)

	var wg sync.WaitGroup

	//nolint:mnd // ignore magic number linters
	wg.Add(2)

	go func() {
		logrus.Debug("Stopping terraform...")

		if err := k.tfRunner.Stop(); err != nil {
			errCh <- fmt.Errorf("error stopping terraform: %w", err)
		}

		wg.Done()
	}()

	go func() {
		logrus.Debug("Stopping awscli...")

		if err := k.awsRunner.Stop(); err != nil {
			errCh <- fmt.Errorf("error stopping awscli: %w", err)
		}

		wg.Done()
	}()

	go func() {
		wg.Wait()
		close(doneCh)
	}()

	select {
	case <-doneCh:

	case err := <-errCh:
		close(errCh)

		return err
	}

	return nil
}

func (k *Kubernetes) checkVPCConnection() error {
	var (
		cidr string
		err  error
	)

	if k.FuryctlConf.Spec.Infrastructure != nil {
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
			return fmt.Errorf(common.SErrWrapWithStr, errCIDRBlockFromVpc, err)
		}
	}

	if k.FuryctlConf.Spec.Kubernetes.ApiServer.PrivateAccess &&
		!k.FuryctlConf.Spec.Kubernetes.ApiServer.PublicAccess &&
		k.FuryctlConf.Spec.Infrastructure != nil &&
		k.FuryctlConf.Spec.Infrastructure.Vpn != nil {
		return k.queryAWSDNSServer(cidr)
	}

	return nil
}

func (*Kubernetes) queryAWSDNSServer(cidr string) error {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf(common.SErrWrapWithStr, errParsingCIDR, err)
	}

	offIPNet, err := netx.AddOffsetToIPNet(ipNet, awsDNSServerIPOffset)
	if err != nil {
		return fmt.Errorf(common.SErrWrapWithStr, errAddingOffsetToIPNet, err)
	}

	err = netx.DNSQuery(offIPNet.IP.String(), "google.com.")
	if err != nil {
		return fmt.Errorf(common.SErrWrapWithStr, errResolvingDNS, err)
	}

	return nil
}
