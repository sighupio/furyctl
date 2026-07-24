// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package config

import (
	"regexp"
	"slices"
	"strings"

	"github.com/Al-Pragliola/go-version"
	"github.com/go-playground/validator/v10"
	"github.com/sirupsen/logrus"
)

const (
	apiVersionString = "^kfd\\.sighup\\.io\\/v\\d+((alpha|beta)\\d+)?$"
	eksVersionString = "^\\d+\\.\\d+$"
)

var (
	apiVersionRegex = regexp.MustCompile(apiVersionString)
	eksVersionRegex = regexp.MustCompile(eksVersionString)
	awsRegions      = []string{
		"af-south-1",
		"ap-east-1",
		"ap-northeast-1",
		"ap-northeast-2",
		"ap-northeast-3",
		"ap-south-1",
		"ap-south-2",
		"ap-southeast-1",
		"ap-southeast-2",
		"ap-southeast-3",
		"ap-southeast-4",
		"ca-central-1",
		"cn-north-1",
		"cn-northwest-1",
		"eu-central-1",
		"eu-central-2",
		"eu-north-1",
		"eu-south-1",
		"eu-south-2",
		"eu-west-1",
		"eu-west-2",
		"eu-west-3",
		"me-central-1",
		"me-south-1",
		"sa-east-1",
		"us-east-1",
		"us-east-2",
		"us-gov-east-1",
		"us-gov-west-1",
		"us-west-1",
		"us-west-2",
	}
)

func NewValidator() *validator.Validate {
	validate := validator.New()

	validations := map[string]validator.Func{
		"api-version":           ValidateAPIVersion,
		"cluster-kind":          ValidateClusterKind,
		"eks-version":           ValidateEksVersion,
		"permissive-semver":     ValidatePermissiveSemVer,
		"permissive-constraint": ValidatePermissiveConstraint,
		"aws-region":            ValidateAwsRegion,
	}

	for name, fn := range validations {
		if err := validate.RegisterValidation(name, fn); err != nil {
			logrus.Warnf("error registering validation %q: %v", name, err)
			return nil
		}
	}

	return validate
}

func ValidateAPIVersion(fl validator.FieldLevel) bool {
	return apiVersionRegex.MatchString(fl.Field().String())
}

func ValidateClusterKind(fl validator.FieldLevel) bool {
	return fl.Field().String() == "EKSCluster" ||
		fl.Field().String() == "KFDDistribution" ||
		fl.Field().String() == "OnPremises" ||
		fl.Field().String() == "Immutable"
}

func ValidatePermissiveSemVer(fl validator.FieldLevel) bool {
	v := strings.TrimPrefix(fl.Field().String(), "v")

	_, err := version.NewVersion(v)

	return err == nil
}

func ValidatePermissiveConstraint(fl validator.FieldLevel) bool {
	c := strings.TrimPrefix(fl.Field().String(), "v")

	_, err := version.NewConstraint(c)

	return err == nil
}

func ValidateEksVersion(fl validator.FieldLevel) bool {
	return eksVersionRegex.MatchString(fl.Field().String())
}

func ValidateAwsRegion(fl validator.FieldLevel) bool {
	reg := strings.ToLower(fl.Field().String())

	return slices.Contains(awsRegions, reg)
}
