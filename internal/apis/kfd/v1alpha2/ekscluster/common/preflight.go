// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package common

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/ekscluster/v1alpha2/private"
	"github.com/sighupio/furyctl/configs"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/ekscluster/vpn"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/tool/awscli"
	"github.com/sighupio/furyctl/internal/tool/furyagent"
	"github.com/sighupio/furyctl/internal/tool/terraform"
	iox "github.com/sighupio/furyctl/internal/x/io"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
	"github.com/sighupio/furyctl/pkg/template"
)

const vpnDefaultPort = 1194

var (
	ErrAWSS3BucketNotFound                   = errors.New("AWS S3 Bucket not found")
	ErrAWSS3BucketRegionMismatch             = errors.New("AWS S3 Bucket region mismatch")
	ErrCannotCreateTerraformStateAWSS3Bucket = errors.New("cannot create terraform state aws s3 bucket")
	ErrEnsureTerraformStateAWSS3Bucket       = errors.New("cannot ensure terraform state aws s3 bucket is present")
	bucketRegex                              = regexp.MustCompile("(?m)bucket\\s*=\\s*\"([^\"]+)\"")
	serverIPRegex                            = regexp.MustCompile("(?m)public_ip\\s*=\\s*\"([^\"]+)\"")
)

type PreFlight struct {
	*cluster.OperationPhase

	FuryctlConf                        private.EksclusterKfdV1Alpha2
	ConfigPath                         string
	AWSRunner                          *awscli.Runner
	VPNConnector                       *vpn.Connector
	TFRunnerInfra                      *terraform.Runner
	InfrastructureTerraformOutputsPath string
}

func (p *PreFlight) Prepare() error {
	if err := p.CreateRootFolder(); err != nil {
		return fmt.Errorf("error creating preflight phase folder: %w", err)
	}

	if err := p.copyFromTemplate(); err != nil {
		return err
	}

	if err := p.CreateTerraformFolderStructure(); err != nil {
		return fmt.Errorf("error creating preflight phase folder structure: %w", err)
	}

	if _, err := os.Stat(path.Join(p.Path, "secrets")); os.IsNotExist(err) {
		if err := os.Mkdir(path.Join(p.Path, "secrets"), iox.FullPermAccess); err != nil {
			return fmt.Errorf("error creating secrets folder: %w", err)
		}
	}

	if _, err := os.Stat(p.InfrastructureTerraformOutputsPath); os.IsNotExist(err) {
		if err := os.MkdirAll(p.InfrastructureTerraformOutputsPath, iox.FullPermAccess); err != nil {
			return fmt.Errorf("error creating infrastructure terraform outputs folder: %w", err)
		}
	}

	return nil
}

func (p *PreFlight) EnsureTerraformStateAWSS3Bucket() error {
	getErr := p.assertTerraformStateAWSS3BucketMatches()
	if getErr == nil {
		return nil
	}

	if errors.Is(getErr, ErrAWSS3BucketNotFound) {
		if err := p.createTerraformStateAWSS3Bucket(); err != nil {
			return fmt.Errorf("%w: %w", ErrEnsureTerraformStateAWSS3Bucket, err)
		}

		return p.assertTerraformStateAWSS3BucketMatches()
	}

	return getErr
}

func (p *PreFlight) assertTerraformStateAWSS3BucketMatches() error {
	r, err := p.AWSRunner.S3Api(
		false,
		"get-bucket-location",
		"--bucket",
		string(p.FuryctlConf.Spec.ToolsConfiguration.Terraform.State.S3.BucketName),
		"--output",
		"text",
	)
	if err != nil {
		return fmt.Errorf(
			"%s: %w",
			string(p.FuryctlConf.Spec.ToolsConfiguration.Terraform.State.S3.BucketName),
			ErrAWSS3BucketNotFound,
		)
	}

	// AWS S3 Bucket in us-east-1 region returns None as LocationConstraint
	//nolint:lll // https://awscli.amazonaws.com/v2/documentation/api/latest/reference/s3api/get-bucket-location.html#output
	if r == "None" {
		r = "us-east-1"
	}

	if r != string(p.FuryctlConf.Spec.ToolsConfiguration.Terraform.State.S3.Region) {
		return fmt.Errorf(
			"%w, expected %s, got %s",
			ErrAWSS3BucketRegionMismatch,
			p.FuryctlConf.Spec.ToolsConfiguration.Terraform.State.S3.Region,
			r,
		)
	}

	return nil
}

func (p *PreFlight) createTerraformStateAWSS3Bucket() error {
	bucket := string(p.FuryctlConf.Spec.ToolsConfiguration.Terraform.State.S3.BucketName)
	region := string(p.FuryctlConf.Spec.ToolsConfiguration.Terraform.State.S3.Region)

	if _, err := p.AWSRunner.S3Api(
		false,
		"create-bucket",
		"--bucket", bucket,
		"--region", region,
		"--create-bucket-configuration", "LocationConstraint="+region,
	); err != nil {
		return fmt.Errorf("%w: %w", ErrCannotCreateTerraformStateAWSS3Bucket, err)
	}

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
					"bucketName":           p.FuryctlConf.Spec.ToolsConfiguration.Terraform.State.S3.BucketName,
					"keyPrefix":            p.FuryctlConf.Spec.ToolsConfiguration.Terraform.State.S3.KeyPrefix,
					"region":               p.FuryctlConf.Spec.ToolsConfiguration.Terraform.State.S3.Region,
					"skipRegionValidation": p.FuryctlConf.Spec.ToolsConfiguration.Terraform.State.S3.SkipRegionValidation,
				},
			},
		},
	}

	cfg.Data = tfConfVars

	err = p.CopyFromTemplate(
		cfg,
		prefix,
		tmpFolder,
		targetTfDir,
		p.ConfigPath,
	)
	if err != nil {
		return fmt.Errorf("error generating from template files: %w", err)
	}

	return nil
}

func (p *PreFlight) IsVPNRequired() bool {
	return p.FuryctlConf.Spec.Infrastructure != nil &&
		p.FuryctlConf.Spec.Infrastructure.Vpn != nil &&
		(p.FuryctlConf.Spec.Infrastructure.Vpn.Instances == nil ||
			p.FuryctlConf.Spec.Infrastructure.Vpn.Instances != nil &&
				*p.FuryctlConf.Spec.Infrastructure.Vpn.Instances > 0) &&
		p.FuryctlConf.Spec.Kubernetes.ApiServer.PrivateAccess &&
		!p.FuryctlConf.Spec.Kubernetes.ApiServer.PublicAccess
}

func (p *PreFlight) HandleVPN() error {
	logrus.Info("VPN required, checking if configuration file exists...")

	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("error getting current dir: %w", err)
	}

	ovpnFileName := p.FuryctlConf.Metadata.Name + ".ovpn"

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

	if err := p.VPNConnector.Connect(); err != nil {
		return fmt.Errorf("error connecting to vpn: %w", err)
	}

	return nil
}

func (p *PreFlight) regenVPNCerts() error {
	if err := p.TFRunnerInfra.Init(); err != nil {
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

	accessKey, err := p.AWSRunner.Configure(true, "get", "aws_access_key_id")
	if err != nil {
		return fmt.Errorf("error getting aws access key: %w", err)
	}

	secretKey, err := p.AWSRunner.Configure(true, "get", "aws_secret_access_key")
	if err != nil {
		return fmt.Errorf("error getting aws secret key: %w", err)
	}

	operatorName := p.getOperatorName()

	furyAgentCfg := furyagent.AgentConfig{
		Storage: furyagent.Storage{
			Provider:     "s3",
			Region:       string(p.FuryctlConf.Spec.Region),
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

	if err := p.VPNConnector.GenerateCertificates(); err != nil {
		return fmt.Errorf("error generating certificates: %w", err)
	}

	return nil
}

func (p *PreFlight) getVPNBucketName() (string, error) {
	out, err := p.TFRunnerInfra.State("show", "module.vpn[0].aws_s3_bucket.furyagent", "-no-color")
	if err != nil {
		return "", fmt.Errorf("error getting vpn bucket name: %w", err)
	}

	bucket := bucketRegex.FindStringSubmatch(out)

	if len(bucket) < 2 { //nolint:mnd // we want to check the length of the regex match
		return "", fmt.Errorf("error getting vpn bucket name: %w", err)
	}

	return bucket[1], nil
}

func (p *PreFlight) getVPNServers() ([]string, error) {
	servers := []string{}
	port := vpnDefaultPort

	if p.FuryctlConf.Spec.Infrastructure.Vpn.Port != nil {
		p := *p.FuryctlConf.Spec.Infrastructure.Vpn.Port

		port = int(p)
	}

	for i := range *p.FuryctlConf.Spec.Infrastructure.Vpn.Instances {
		out, err := p.TFRunnerInfra.State("show", fmt.Sprintf("module.vpn[0].aws_eip.vpn[%d]", i), "-no-color")
		if err != nil {
			return servers, fmt.Errorf("error getting vpn instance: %w", err)
		}

		servers = append(servers, serverIPRegex.FindStringSubmatch(out)[1]+":"+strconv.Itoa(port))
	}

	return servers, nil
}

func (p *PreFlight) getOperatorName() string {
	operatorName := "sighup"

	if p.FuryctlConf.Spec.Infrastructure.Vpn.OperatorName != nil {
		operatorName = *p.FuryctlConf.Spec.Infrastructure.Vpn.OperatorName
	}

	return operatorName
}
