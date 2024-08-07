// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dependencies

import (
	"errors"
	"fmt"

	"github.com/sighupio/furyctl/internal/dependencies/envvars"
	"github.com/sighupio/furyctl/internal/dependencies/tools"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	dist "github.com/sighupio/furyctl/pkg/distribution"
)

var (
	errValidatingTools = errors.New("errors validating tools")
	errValidatingEnv   = errors.New("errors validating env vars")
)

func NewValidator(executor execx.Executor, binPath, furyctlPath string, autoConnect bool) *Validator {
	return &Validator{
		toolsValidator:   tools.NewValidator(executor, binPath, furyctlPath, autoConnect),
		envVarsValidator: envvars.NewValidator(),
	}
}

type Validator struct {
	toolsValidator   *tools.Validator
	envVarsValidator *envvars.Validator
}

func (v *Validator) ValidateBaseReqs() error {
	if _, errs := v.toolsValidator.ValidateBaseReqs(); len(errs) > 0 {
		return fmt.Errorf("%w: %v", errValidatingTools, errs)
	}

	return nil
}

func (v *Validator) Validate(res dist.DownloadResult) error {
	if _, errs := v.toolsValidator.Validate(
		res.DistroManifest,
		res.MinimalConf,
	); len(errs) > 0 {
		return fmt.Errorf("%w: %v", errValidatingTools, errs)
	}

	if _, errs := v.envVarsValidator.Validate(res.MinimalConf.Kind); len(errs) > 0 {
		return fmt.Errorf("%w: %v", errValidatingEnv, errs)
	}

	return nil
}
