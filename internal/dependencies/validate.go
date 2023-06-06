// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dependencies

import (
	"errors"
	"fmt"

	"github.com/sighupio/furyctl/internal/dependencies/envvars"
	"github.com/sighupio/furyctl/internal/dependencies/tools"
	"github.com/sighupio/furyctl/internal/dependencies/toolsconf"
	"github.com/sighupio/furyctl/internal/distribution"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

var (
	errValidatingTools     = errors.New("errors validating tools")
	errValidatingEnv       = errors.New("errors validating env vars")
	errValidatingToolsConf = errors.New("errors validating tools configuration")
)

func NewValidator(executor execx.Executor, binPath, furyctlPath string, autoConnect bool) *Validator {
	return &Validator{
		toolsValidator:   tools.NewValidator(executor, binPath, furyctlPath, autoConnect),
		envVarsValidator: envvars.NewValidator(),
		infraValidator:   toolsconf.NewValidator(executor),
	}
}

type Validator struct {
	toolsValidator   *tools.Validator
	envVarsValidator *envvars.Validator
	infraValidator   *toolsconf.Validator
}

func (v *Validator) ValidateBaseReqs() error {
	if _, errs := v.toolsValidator.ValidateBaseReqs(); len(errs) > 0 {
		return fmt.Errorf("%w: %v", errValidatingTools, errs)
	}

	return nil
}

func (v *Validator) Validate(res distribution.DownloadResult) error {
	if _, errs := v.toolsValidator.Validate(
		res.DistroManifest,
		res.MinimalConf,
	); len(errs) > 0 {
		return fmt.Errorf("%w: %v", errValidatingTools, errs)
	}

	if _, errs := v.envVarsValidator.Validate(res.MinimalConf.Kind); len(errs) > 0 {
		return fmt.Errorf("%w: %v", errValidatingEnv, errs)
	}

	if res.MinimalConf.Spec.ToolsConfiguration != nil {
		if _, errs := v.infraValidator.Validate(
			res.MinimalConf.Spec.ToolsConfiguration.Terraform.State.S3,
		); len(errs) > 0 {
			return fmt.Errorf("%w: %v", errValidatingToolsConf, errs)
		}
	}

	return nil
}
