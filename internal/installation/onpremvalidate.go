// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package installation

import (
	"fmt"
	"path/filepath"

	"github.com/sighupio/fury-distribution/pkg/apis/onpremises/v1alpha2/public"

	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

type OnPremisesInstallationValidator struct {
	furyctlContent string
	repoPath       string
	furyctl        public.OnpremisesKfdV1Alpha2
}

// Run all tests from the installed modules.
func (v *OnPremisesInstallationValidator) Validate() error {
	v.furyctl = public.OnpremisesKfdV1Alpha2{}
	if err := yamlx.UnmarshalV3([]byte(v.furyctlContent), &v.furyctl); err != nil {
		return fmt.Errorf("%w: %w", ErrReadingSpec, err)
	}

	if err := v.validateInstaller(); err != nil {
		return fmt.Errorf("%w: %w", ErrReadingSpec, err)
	}

	distribution := KFDInstallationValidator{
		furyctlContent: v.furyctlContent,
		repoPath:       v.repoPath,
	}

	return distribution.Validate()
}

func (v *OnPremisesInstallationValidator) validateInstaller() error {
	return RunTests("onprem", filepath.Join(v.repoPath, v.furyctl.Metadata.Name, "vendor", "installers", "onpremises"))
}
