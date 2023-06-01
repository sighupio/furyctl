// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package toolsconf

import (
	"errors"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/furyctl/internal/tool/awscli"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

var (
	ErrAWSS3BucketNotFound       = errors.New("AWS S3 Bucket not found, please create it before running furyctl")
	ErrAWSS3BucketRegionMismatch = errors.New("AWS S3 Bucket region mismatch")
)

func NewValidator(executor execx.Executor) *Validator {
	return &Validator{
		awsCliRunner: awscli.NewRunner(
			executor,
			awscli.Paths{
				Awscli:  "aws",
				WorkDir: "",
			},
		),
	}
}

type Validator struct {
	awsCliRunner *awscli.Runner
}

func (v *Validator) Validate(s3Conf config.ToolsConfigurationTerrraformStateS3) ([]string, []error) {
	return v.checkAWSS3Bucket(s3Conf.BucketName, s3Conf.Region)
}

func (v *Validator) checkAWSS3Bucket(bucketName, region string) ([]string, []error) {
	oks := make([]string, 0)
	errs := make([]error, 0)

	r, err := v.awsCliRunner.S3Api("get-bucket-location", "--bucket", bucketName, "--output", "text")
	if err != nil {
		logrus.Debug(fmt.Errorf("error checking AWS S3 Bucket: %w", err))

		errs = append(errs, ErrAWSS3BucketNotFound)

		return oks, errs
	}

	// AWS S3 Bucket in us-east-1 region returns None as LocationConstraint
	// https://awscli.amazonaws.com/v2/documentation/api/latest/reference/s3api/get-bucket-location.html#output
	if r == "None" {
		r = "us-east-1"
	}

	if r != region {
		errs = append(errs, fmt.Errorf("%w, expected %s, got %s", ErrAWSS3BucketRegionMismatch, region, r))

		return oks, errs
	}

	oks = append(oks, "AWS S3 Bucket")

	return oks, errs
}
