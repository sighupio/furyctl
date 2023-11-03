// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package create

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"

	r3diff "github.com/r3labs/diff/v3"
	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/fury-distribution/pkg/apis/ekscluster/v1alpha2/private"
	"github.com/sighupio/furyctl/configs"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/eks/rules"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/eks/vpn"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/diffs"
	"github.com/sighupio/furyctl/internal/state"
	"github.com/sighupio/furyctl/internal/template"
	"github.com/sighupio/furyctl/internal/tool/awscli"
	"github.com/sighupio/furyctl/internal/tool/furyagent"
	"github.com/sighupio/furyctl/internal/tool/kubectl"
	"github.com/sighupio/furyctl/internal/tool/terraform"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

const vpnDefaultPort = 1194

var (
	errImmutable  = errors.New("immutable path changed")
	bucketRegex   = regexp.MustCompile("(?m)bucket\\s*=\\s*\"([^\"]+)\"")
	serverIPRegex = regexp.MustCompile("(?m)public_ip\\s*=\\s*\"([^\"]+)\"")
)

type PreFlight struct {
	*cluster.OperationPhase
	furyctlConf     private.EksclusterKfdV1Alpha2
	stateStore      state.Storer
	distroPath      string
	furyctlConfPath string
	kubeconfig      string
	tfRunnerKube    *terraform.Runner
	tfRunnerInfra   *terraform.Runner
	vpnConnector    *vpn.Connector
	kubeRunner      *kubectl.Runner
	awsRunner       *awscli.Runner
	dryRun          bool
}

func NewPreFlight(
	furyctlConf private.EksclusterKfdV1Alpha2,
	kfdManifest config.KFD,
	paths cluster.CreatorPaths,
	dryRun bool,
	vpnAutoConnect bool,
	skipVpn bool,
) (*PreFlight, error) {
	var vpnConfig *private.SpecInfrastructureVpn

	if furyctlConf.Spec.Infrastructure != nil {
		vpnConfig = furyctlConf.Spec.Infrastructure.Vpn
	}

	preFlightDir := path.Join(paths.WorkDir, cluster.OperationPhasePreFlight)

	phase, err := cluster.NewOperationPhase(preFlightDir, kfdManifest.Tools, paths.BinPath)
	if err != nil {
		return nil, fmt.Errorf("error creating preflight phase: %w", err)
	}

	kubeconfig := path.Join(phase.Path, "secrets", "kubeconfig")

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
		OperationPhase: phase,
		furyctlConf:    furyctlConf,
		stateStore: state.NewStore(
			paths.DistroPath,
			paths.ConfigPath,
			kubeconfig,
			paths.WorkDir,
			kfdManifest.Tools.Common.Kubectl.Version,
			paths.BinPath,
		),
		distroPath:      paths.DistroPath,
		furyctlConfPath: paths.ConfigPath,
		tfRunnerKube: terraform.NewRunner(
			execx.NewStdExecutor(),
			terraform.Paths{
				WorkDir:   path.Join(phase.Path, "terraform", "kubernetes"),
				Terraform: phase.TerraformPath,
			},
		),
		tfRunnerInfra: terraform.NewRunner(
			execx.NewStdExecutor(),
			terraform.Paths{
				WorkDir:   path.Join(phase.Path, "terraform", "infrastructure"),
				Terraform: phase.TerraformPath,
			},
		),
		kubeRunner: kubectl.NewRunner(
			execx.NewStdExecutor(),
			kubectl.Paths{
				Kubectl:    phase.KubectlPath,
				WorkDir:    phase.Path,
				Kubeconfig: kubeconfig,
			},
			true,
			true,
			false,
		),
		awsRunner: awscli.NewRunner(
			execx.NewStdExecutor(),
			awscli.Paths{
				Awscli:  "aws",
				WorkDir: paths.WorkDir,
			},
		),
		vpnConnector: vpnConnector,
		kubeconfig:   kubeconfig,
		dryRun:       dryRun,
	}, nil
}

func (p *PreFlight) Exec() error {
	logrus.Info("Running preflight checks")

	if err := p.CreateFolder(); err != nil {
		return fmt.Errorf("error creating preflight phase folder: %w", err)
	}

	if err := p.copyFromTemplate(); err != nil {
		return err
	}

	if err := p.CreateFolderStructure(); err != nil {
		return fmt.Errorf("error creating preflight phase folder structure: %w", err)
	}

	if _, err := os.Stat(path.Join(p.Path, "secrets")); os.IsNotExist(err) {
		if err := os.Mkdir(path.Join(p.Path, "secrets"), iox.FullPermAccess); err != nil {
			return fmt.Errorf("error creating secrets folder: %w", err)
		}
	}

	if err := p.tfRunnerKube.Init(); err != nil {
		return fmt.Errorf("error running terraform init: %w", err)
	}

	if _, err := p.tfRunnerKube.State("show", "data.aws_eks_cluster.fury"); err != nil {
		logrus.Debug("Cluster does not exist, skipping state checks")

		logrus.Info("Preflight checks completed successfully")

		return nil //nolint:nilerr // we want to return nil here
	}

	logrus.Info("Updating kubeconfig...")

	if _, err := p.awsRunner.Eks(
		false,
		"update-kubeconfig",
		"--name",
		p.furyctlConf.Metadata.Name,
		"--kubeconfig",
		p.kubeconfig,
		"--region",
		string(p.furyctlConf.Spec.Region),
	); err != nil {
		return fmt.Errorf("error updating kubeconfig: %w", err)
	}

	if p.isVPNRequired() {
		if err := p.handleVPN(); err != nil {
			return fmt.Errorf("error handling vpn: %w", err)
		}
	}

	logrus.Info("Checking that the cluster is reachable...")

	if _, err := p.kubeRunner.Version(); err != nil {
		return fmt.Errorf("cluster is unreachable, make sure you have access to the cluster: %w", err)
	}

	diffChecker, err := p.CreateDiffChecker()
	if err != nil {
		return fmt.Errorf("error creating diff checker: %w", err)
	}

	d, err := diffChecker.GenerateDiff()
	if err != nil {
		return fmt.Errorf("error while generating diff: %w", err)
	}

	if len(d) > 0 {
		logrus.Infof(
			"Differences found from previous cluster configuration:\n%s",
			diffChecker.DiffToString(d),
		)

		logrus.Warn("Cluster configuration has changed, checking for immutable violations...")

		if err := p.CheckStateDiffs(d, diffChecker); err != nil {
			return fmt.Errorf("error checking state diffs: %w", err)
		}
	}

	logrus.Info("Preflight checks completed successfully")

	return nil
}

func (p *PreFlight) copyFromTemplate() error {
	var cfg template.Config

	tmpFolder, err := os.MkdirTemp("", "furyctl-kube-configs-")
	if err != nil {
		return fmt.Errorf("error creating temp folder: %w", err)
	}

	defer os.RemoveAll(tmpFolder)

	subFS, err := fs.Sub(configs.Tpl, path.Join("provisioners", "preflight", "aws"))
	if err != nil {
		return fmt.Errorf("error getting subfs: %w", err)
	}

	err = iox.CopyRecursive(subFS, tmpFolder)
	if err != nil {
		return fmt.Errorf("error copying template files: %w", err)
	}

	targetTfDir := path.Join(p.Path, "terraform")
	prefix := "kube"

	tfConfVars := map[string]map[any]any{
		"terraform": {
			"backend": map[string]any{
				"s3": map[string]any{
					"bucketName":           p.furyctlConf.Spec.ToolsConfiguration.Terraform.State.S3.BucketName,
					"keyPrefix":            p.furyctlConf.Spec.ToolsConfiguration.Terraform.State.S3.KeyPrefix,
					"region":               p.furyctlConf.Spec.ToolsConfiguration.Terraform.State.S3.Region,
					"skipRegionValidation": p.furyctlConf.Spec.ToolsConfiguration.Terraform.State.S3.SkipRegionValidation,
				},
			},
		},
	}

	cfg.Data = tfConfVars

	err = p.OperationPhase.CopyFromTemplate(
		cfg,
		prefix,
		tmpFolder,
		targetTfDir,
		p.furyctlConfPath,
	)
	if err != nil {
		return fmt.Errorf("error generating from template files: %w", err)
	}

	return nil
}

func (p *PreFlight) handleVPN() error {
	logrus.Info("VPN required, checking if configuration file exists...")

	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("error getting current dir: %w", err)
	}

	ovpnFileName := fmt.Sprintf("%s.ovpn", p.furyctlConf.Metadata.Name)

	ovpnPath, err := filepath.Abs(path.Join(wd, ovpnFileName))
	if err != nil {
		return fmt.Errorf("error getting ovpn absolute path: %w", err)
	}

	if _, err := os.Stat(ovpnPath); err != nil {
		logrus.Info("No ovpn file found, generating it...")

		if err := p.regenVPNCerts(); err != nil {
			return fmt.Errorf("error regenerating vpn certs: %w", err)
		}
	}

	if err := p.vpnConnector.Connect(); err != nil {
		return fmt.Errorf("error connecting to vpn: %w", err)
	}

	return nil
}

func (p *PreFlight) getVPNBucketName() (string, error) {
	out, err := p.tfRunnerInfra.State("show", "module.vpn[0].aws_s3_bucket.furyagent", "-no-color")
	if err != nil {
		return "", fmt.Errorf("error getting vpn bucket name: %w", err)
	}

	bucket := bucketRegex.FindStringSubmatch(out)

	if len(bucket) < 2 { //nolint:gomnd // we want to check the length of the regex match
		return "", fmt.Errorf("error getting vpn bucket name: %w", err)
	}

	return bucket[1], nil
}

func (p *PreFlight) getVPNServers() ([]string, error) {
	servers := []string{}
	port := vpnDefaultPort

	if p.furyctlConf.Spec.Infrastructure.Vpn.Port != nil {
		p := *p.furyctlConf.Spec.Infrastructure.Vpn.Port

		port = int(p)
	}

	for i := 0; i < *p.furyctlConf.Spec.Infrastructure.Vpn.Instances; i++ {
		out, err := p.tfRunnerInfra.State("show", fmt.Sprintf("module.vpn[0].aws_eip.vpn[%d]", i), "-no-color")
		if err != nil {
			return servers, fmt.Errorf("error getting vpn instance: %w", err)
		}

		servers = append(servers, serverIPRegex.FindStringSubmatch(out)[1]+":"+strconv.Itoa(port))
	}

	return servers, nil
}

func (p *PreFlight) getOperatorName() string {
	operatorName := "sighup"

	if p.furyctlConf.Spec.Infrastructure.Vpn.OperatorName != nil {
		operatorName = *p.furyctlConf.Spec.Infrastructure.Vpn.OperatorName
	}

	return operatorName
}

func (p *PreFlight) regenVPNCerts() error {
	if err := p.tfRunnerInfra.Init(); err != nil {
		return fmt.Errorf("error running terraform init: %w", err)
	}

	bucketName, err := p.getVPNBucketName()
	if err != nil {
		return fmt.Errorf("error getting vpn bucket name: %w", err)
	}

	servers, err := p.getVPNServers()
	if err != nil {
		return fmt.Errorf("error getting vpn servers: %w", err)
	}

	accessKey, err := p.awsRunner.Configure(true, "get", "aws_access_key_id")
	if err != nil {
		return fmt.Errorf("error getting aws access key: %w", err)
	}

	secretKey, err := p.awsRunner.Configure(true, "get", "aws_secret_access_key")
	if err != nil {
		return fmt.Errorf("error getting aws secret key: %w", err)
	}

	operatorName := p.getOperatorName()

	furyAgentCfg := furyagent.AgentConfig{
		Storage: furyagent.Storage{
			Provider:     "s3",
			Region:       string(p.furyctlConf.Spec.Region),
			BucketName:   bucketName,
			AwsAccessKey: accessKey,
			AwsSecretKey: secretKey,
		},
		ClusterComponent: furyagent.ClusterComponent{
			OpenVPN: furyagent.OpenVPN{
				CertDir: "/etc/openvpn/pki",
				Servers: servers,
			},
			SSHKeys: furyagent.SSHKeys{
				User:            operatorName,
				TempDir:         "/var/lib/SIGHUP/tmp",
				LocalDirConfigs: ".",
				Adapter: furyagent.Adapter{
					Name: "github",
				},
			},
		},
	}

	furyAgentFile, err := yamlx.MarshalV3(furyAgentCfg)
	if err != nil {
		return fmt.Errorf("error marshalling furyagent config: %w", err)
	}

	if err := iox.WriteFile(path.Join(p.Path, "secrets", "furyagent.yml"), furyAgentFile); err != nil {
		return fmt.Errorf("error writing furyagent config: %w", err)
	}

	if err := p.vpnConnector.GenerateCertificates(); err != nil {
		return fmt.Errorf("error generating certificates: %w", err)
	}

	return nil
}

func (p *PreFlight) isVPNRequired() bool {
	return p.furyctlConf.Spec.Infrastructure != nil &&
		p.furyctlConf.Spec.Infrastructure.Vpn != nil &&
		(p.furyctlConf.Spec.Infrastructure.Vpn.Instances == nil ||
			p.furyctlConf.Spec.Infrastructure.Vpn.Instances != nil &&
				*p.furyctlConf.Spec.Infrastructure.Vpn.Instances > 0) &&
		p.furyctlConf.Spec.Kubernetes.ApiServer.PrivateAccess &&
		!p.furyctlConf.Spec.Kubernetes.ApiServer.PublicAccess
}

func (p *PreFlight) CreateDiffChecker() (diffs.Checker, error) {
	storedCfg := map[string]any{}

	storedCfgStr, err := p.stateStore.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("error while getting current cluster config: %w", err)
	}

	if err = yamlx.UnmarshalV3(storedCfgStr, &storedCfg); err != nil {
		return nil, fmt.Errorf("error while unmarshalling config file: %w", err)
	}

	newCfg, err := yamlx.FromFileV3[map[string]any](p.furyctlConfPath)
	if err != nil {
		return nil, fmt.Errorf("error while reading config file: %w", err)
	}

	return diffs.NewBaseChecker(storedCfg, newCfg), nil
}

func (*PreFlight) GenerateDiffs(diffChecker diffs.Checker) (r3diff.Changelog, error) {
	d, err := diffChecker.GenerateDiff()
	if err != nil {
		return nil, fmt.Errorf("error while diffing configs: %w", err)
	}

	return d, nil
}

func (p *PreFlight) CheckStateDiffs(d r3diff.Changelog, diffChecker diffs.Checker) error {
	var errs []error

	r, err := rules.NewEKSClusterRulesExtractor(p.distroPath)
	if err != nil {
		if !errors.Is(err, rules.ErrReadingRulesFile) {
			return fmt.Errorf("error while creating rules builder: %w", err)
		}

		logrus.Warn("No rules file found, skipping immutable checks")

		return nil
	}

	errs = append(errs, diffChecker.AssertImmutableViolations(d, r.GetImmutables("infrastructure"))...)
	errs = append(errs, diffChecker.AssertImmutableViolations(d, r.GetImmutables("kubernetes"))...)
	errs = append(errs, diffChecker.AssertImmutableViolations(d, r.GetImmutables("distribution"))...)

	if len(errs) > 0 {
		return fmt.Errorf("%w: %s", errImmutable, errs)
	}

	return nil
}