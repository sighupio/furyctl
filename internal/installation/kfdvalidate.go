// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package installation

import (
	"fmt"
	"path/filepath"

	"github.com/sighupio/fury-distribution/pkg/apis/kfddistribution/v1alpha2/public"
	"github.com/sirupsen/logrus"

	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

type KFDInstallationValidator struct {
	furyctlContent string
	repoPath       string
	furyctl        public.KfddistributionKfdV1Alpha2
}

// Run all tests from the installed modules.
func (v *KFDInstallationValidator) Validate() error {
	v.furyctl = public.KfddistributionKfdV1Alpha2{}
	if err := yamlx.UnmarshalV3([]byte(v.furyctlContent), &v.furyctl); err != nil {
		return fmt.Errorf("%w: %w", ErrReadingSpec, err)
	}

	hasErrors := false

	if err := v.validateIntegration(); err != nil {
		logrus.Error(err)
		hasErrors = true
	}
	if err := v.validateMonitoring(); err != nil {
		logrus.Error(err)
		hasErrors = true
	}
	if err := v.validateLogging(); err != nil {
		logrus.Error(err)
		hasErrors = true
	}
	if err := v.validateIngress(); err != nil {
		logrus.Error(err)
		hasErrors = true
	}
	if err := v.validateNetworking(); err != nil {
		logrus.Error(err)
		hasErrors = true
	}
	if err := v.validatePolicy(); err != nil {
		logrus.Error(err)
		hasErrors = true
	}
	if err := v.validateTracing(); err != nil {
		logrus.Error(err)
		hasErrors = true
	}
	if err := v.validateDR(); err != nil {
		logrus.Error(err)
		hasErrors = true
	}

	if hasErrors {
		return ErrValidating
	}

	return nil
}

func (v *KFDInstallationValidator) validateIntegration() error {
	// return RunTests(filepath.Join(v.repoPath, v.furyctl.Metadata.Name, "vendor", "modules", ""))
	return nil
}

func (v *KFDInstallationValidator) validateMonitoring() error {
	if v.furyctl.Spec.Distribution.Modules.Monitoring.Type == "none" {
		return nil
	}
	return RunTests("monitoring", filepath.Join(v.repoPath, v.furyctl.Metadata.Name, "vendor", "modules", "monitoring"))
}

func (v *KFDInstallationValidator) validateLogging() error {
	if v.furyctl.Spec.Distribution.Modules.Logging.Type == "none" {
		return nil
	}
	return RunTests("logging", filepath.Join(v.repoPath, v.furyctl.Metadata.Name, "vendor", "modules", "logging"))
}

func (v *KFDInstallationValidator) validateIngress() error {
	if v.furyctl.Spec.Distribution.Modules.Ingress.Nginx.Type == "none" {
		return nil
	}
	return RunTests("ingress", filepath.Join(v.repoPath, v.furyctl.Metadata.Name, "vendor", "modules", "ingress"))
}

func (v *KFDInstallationValidator) validateNetworking() error {
	if v.furyctl.Spec.Distribution.Modules.Networking.Type == "none" {
		return nil
	}
	return RunTests("networking", filepath.Join(v.repoPath, v.furyctl.Metadata.Name, "vendor", "modules", "networking"))
}

func (v *KFDInstallationValidator) validatePolicy() error {

	if v.furyctl.Spec.Distribution.Modules.Policy.Type == "none" {
		return nil
	}
	return RunTests("policy", filepath.Join(v.repoPath, v.furyctl.Metadata.Name, "vendor", "modules", "policy"))
}

func (v *KFDInstallationValidator) validateTracing() error {
	if v.furyctl.Spec.Distribution.Modules.Tracing.Type == "none" {
		return nil
	}
	return RunTests("tracing", filepath.Join(v.repoPath, v.furyctl.Metadata.Name, "vendor", "modules", "tracing"))
}

func (v *KFDInstallationValidator) validateDR() error {
	if v.furyctl.Spec.Distribution.Modules.Dr.Type == "none" {
		return nil
	}
	return RunTests("dr", filepath.Join(v.repoPath, v.furyctl.Metadata.Name, "vendor", "modules", "dr"))
}
