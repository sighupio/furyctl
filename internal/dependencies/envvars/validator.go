// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package envvars

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

var (
	ErrMissingEnvVars        = errors.New("missing environment variables")
	ErrMissingRequiredEnvVar = errors.New("missing required environment variable")
)

func NewValidator() *Validator {
	return &Validator{}
}

type Validator struct{}

func (ev *Validator) Validate(kind string) ([]string, []error) {
	if kind == "EKSCluster" {
		return ev.checkEKSCluster()
	}

	return nil, nil
}

func (*Validator) checkEKSCluster() ([]string, []error) {
	oks := make([]string, 0)
	errs := make([]error, 0)

	var otherAwsVars []string

	if os.Getenv("AWS_DEFAULT_REGION") == "" {
		errs = append(errs, fmt.Errorf("%w AWS_DEFAULT_REGION", ErrMissingRequiredEnvVar))
	} else {
		oks = append(oks, "AWS_DEFAULT_REGION")
	}

	if os.Getenv("AWS_PROFILE") != "" {
		oks = append(oks, "AWS_PROFILE")

		return oks, errs
	}

	if os.Getenv("AWS_ACCESS_KEY_ID") == "" {
		otherAwsVars = append(otherAwsVars, "AWS_ACCESS_KEY_ID")
	} else {
		oks = append(oks, "AWS_ACCESS_KEY_ID")
	}

	if os.Getenv("AWS_SECRET_ACCESS_KEY") == "" {
		otherAwsVars = append(otherAwsVars, "AWS_SECRET_ACCESS_KEY")
	} else {
		oks = append(oks, "AWS_SECRET_ACCESS_KEY")
	}

	if len(otherAwsVars) > 0 {
		errs = append(errs, fmt.Errorf("%w, either AWS_Profile or the following: %s",
			ErrMissingEnvVars, strings.Join(otherAwsVars, ", ")))

		return oks, errs
	}

	return oks, errs
}
