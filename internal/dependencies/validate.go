// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dependencies

import (
	"errors"
	"fmt"

	"github.com/sighupio/furyctl/internal/dependencies/envvars"
	"github.com/sighupio/furyctl/internal/dependencies/tools"
	"github.com/sighupio/furyctl/internal/distribution"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

var (
	errValidatingTools = errors.New("errors validating tools")
	errValidatingEnv   = errors.New("errors validating env vars")
)

func NewValidator(executor execx.Executor, binPath string) *Validator {
	return &Validator{
		toolsValidator:   tools.NewValidator(executor, binPath),
		envVarsValidator: envvars.NewValidator(),
	}
}

type Validator struct {
	toolsValidator   *tools.Validator
	envVarsValidator *envvars.Validator
}

func (v *Validator) Validate(res distribution.DownloadResult) error {
	if errs := v.toolsValidator.Validate(res.DistroManifest); len(errs) > 0 {
		return fmt.Errorf("%w: %v", errValidatingTools, errs)
	}

	if errs := v.envVarsValidator.Validate(res.MinimalConf.Kind); len(errs) > 0 {
		return fmt.Errorf("%w: %v", errValidatingEnv, errs)
	}

	return nil
}
