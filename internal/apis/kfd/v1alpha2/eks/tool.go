// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eks

import (
	"errors"
	"fmt"

	"github.com/sighupio/fury-distribution/pkg/apis/ekscluster/v1alpha2/private"
	"github.com/sighupio/furyctl/internal/tool/awscli"
	"github.com/sighupio/furyctl/internal/tool/openvpn"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

var (
	ErrOpenVPNNotInstalled       = errors.New("openvpn is not installed")
	ErrAWSS3BucketNotFound       = errors.New("AWS S3 Bucket not found, please create it before running furyctl")
	ErrAWSS3BucketRegionMismatch = errors.New("AWS S3 Bucket region mismatch")
)

type ExtraToolsValidator struct {
	executor    execx.Executor
	autoConnect bool
}

func NewExtraToolsValidator(executor execx.Executor, autoConnect bool) *ExtraToolsValidator {
	return &ExtraToolsValidator{
		executor:    executor,
		autoConnect: autoConnect,
	}
}

func (x *ExtraToolsValidator) Validate(confPath string) ([]string, []error) {
	var (
		oks  []string
		errs []error
	)

	furyctlConf, err := yamlx.FromFileV3[private.EksclusterKfdV1Alpha2](confPath)
	if err != nil {
		return oks, append(errs, err)
	}

	if err := x.openVPN(furyctlConf); err != nil {
		errs = append(errs, err)
	} else {
		oks = append(oks, "openvpn")
	}

	if err := x.terraformStateAWSS3Bucket(furyctlConf); err != nil {
		errs = append(errs, err)
	} else {
		oks = append(oks, "terraform state aws s3 bucket")
	}

	return oks, errs
}

func (x *ExtraToolsValidator) openVPN(conf private.EksclusterKfdV1Alpha2) error {
	if conf.Spec.Infrastructure != nil &&
		conf.Spec.Infrastructure.Vpn != nil &&
		(conf.Spec.Infrastructure.Vpn.Instances == nil ||
			(conf.Spec.Infrastructure.Vpn.Instances != nil &&
				*conf.Spec.Infrastructure.Vpn.Instances > 0)) &&
		x.autoConnect {
		oRunner := openvpn.NewRunner(x.executor, openvpn.Paths{
			Openvpn: "openvpn",
		})

		if _, err := oRunner.Version(); err != nil {
			return ErrOpenVPNNotInstalled
		}
	}

	return nil
}

func (x *ExtraToolsValidator) terraformStateAWSS3Bucket(conf private.EksclusterKfdV1Alpha2) error {
	awsCliRunner := awscli.NewRunner(
		x.executor,
		awscli.Paths{
			Awscli:  "aws",
			WorkDir: "",
		},
	)

	r, err := awsCliRunner.S3Api(
		false,
		"get-bucket-location",
		"--bucket",
		string(conf.Spec.ToolsConfiguration.Terraform.State.S3.BucketName),
		"--output",
		"text",
	)
	if err != nil {
		return ErrAWSS3BucketNotFound
	}

	// AWS S3 Bucket in us-east-1 region returns None as LocationConstraint
	//nolint:lll // https://awscli.amazonaws.com/v2/documentation/api/latest/reference/s3api/get-bucket-location.html#output
	if r == "None" {
		r = "us-east-1"
	}

	if r != string(conf.Spec.ToolsConfiguration.Terraform.State.S3.Region) {
		return fmt.Errorf(
			"%w, expected %s, got %s",
			ErrAWSS3BucketRegionMismatch,
			conf.Spec.ToolsConfiguration.Terraform.State.S3.Region,
			r,
		)
	}

	return nil
}
