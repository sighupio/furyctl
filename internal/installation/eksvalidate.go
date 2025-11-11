// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package installation

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/sighupio/fury-distribution/pkg/apis/ekscluster/v1alpha2/public"

	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

type EKSInstallationValidator struct {
	furyctlContent string
	repoPath       string
	furyctl        public.EksclusterKfdV1Alpha2
}

var (
	ErrValidatingEKS = errors.New("error validating eks cluster")
)

// Run all tests from the installed modules.
func (v *EKSInstallationValidator) Validate() error {
	v.furyctl = public.EksclusterKfdV1Alpha2{}
	if err := yamlx.UnmarshalV3([]byte(v.furyctlContent), &v.furyctl); err != nil {
		return fmt.Errorf("%w: %w", ErrReadingSpec, err)
	}

	if err := v.validateInstaller(); err != nil {
		return fmt.Errorf("%w: %w", ErrValidatingEKS, err)
	}

	distribution := KFDInstallationValidator{
		furyctlContent: v.furyctlContent,
		repoPath:       v.repoPath,
	}

	return distribution.Validate()
}

func (v *EKSInstallationValidator) validateInstaller() error {
	return RunTests("eks", filepath.Join(v.repoPath, v.furyctl.Metadata.Name, "vendor", "installers", "eks"))
}
