// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package envvars

import (
	"errors"
	"fmt"
	"os"
)

var ErrMissingEnvVar = errors.New("missing environment variable")

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

	if os.Getenv("AWS_ACCESS_KEY_ID") == "" {
		errs = append(errs, fmt.Errorf("AWS_ACCESS_KEY_ID: %w", ErrMissingEnvVar))
	} else {
		oks = append(oks, "AWS_ACCESS_KEY_ID")
	}

	if os.Getenv("AWS_SECRET_ACCESS_KEY") == "" {
		errs = append(errs, fmt.Errorf("AWS_SECRET_ACCESS_KEY: %w", ErrMissingEnvVar))
	} else {
		oks = append(oks, "AWS_SECRET_ACCESS_KEY")
	}

	if os.Getenv("AWS_DEFAULT_REGION") == "" {
		errs = append(errs, fmt.Errorf("AWS_DEFAULT_REGION: %w", ErrMissingEnvVar))
	} else {
		oks = append(oks, "AWS_DEFAULT_REGION")
	}

	return oks, errs
}
