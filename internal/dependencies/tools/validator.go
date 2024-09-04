// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tools

import (
	"errors"
	"reflect"
	"strings"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/furyctl/internal/apis"
	"github.com/sighupio/furyctl/internal/distribution"
	itool "github.com/sighupio/furyctl/internal/tool"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

var ErrWrongToolVersion = errors.New("wrong tool version")

func NewValidator(executor execx.Executor, binPath, furyctlPath string, autoConnect bool) *Validator {
	return &Validator{
		executor: executor,
		toolFactory: NewFactory(executor, FactoryPaths{
			Bin: binPath,
		}),
		furyctlPath: furyctlPath,
		autoConnect: autoConnect,
	}
}

type Validator struct {
	executor    execx.Executor
	toolFactory *Factory
	furyctlPath string
	autoConnect bool
}

func (tv *Validator) ValidateBaseReqs() ([]string, []error) {
	var (
		oks  []string
		errs []error
	)

	sed := tv.toolFactory.Create(itool.Sed, "*")
	if err := sed.CheckBinVersion(); err != nil {
		errs = append(errs, err)
	} else {
		oks = append(oks, "sed")
	}

	git := tv.toolFactory.Create(itool.Git, "*")
	if err := git.CheckBinVersion(); err != nil {
		errs = append(errs, err)
	} else {
		oks = append(oks, "git")
	}

	shell := tv.toolFactory.Create(itool.Shell, "*")
	if err := shell.CheckBinVersion(); err != nil {
		errs = append(errs, err)
	} else {
		oks = append(oks, "shell")
	}

	return oks, errs
}

func (tv *Validator) Validate(kfdManifest config.KFD, miniConf config.Furyctl) ([]string, []error) {
	var (
		oks  []string
		errs []error
	)

	// Validate common tools.
	cOks, cErrs := tv.validateTools(kfdManifest.Tools.Common, kfdManifest)
	oks = append(oks, cOks...)
	errs = append(errs, cErrs...)

	// Validate eks tools only if kind is EKSCluster.
	if miniConf.Kind == "EKSCluster" {
		cOks, cErrs := tv.validateTools(kfdManifest.Tools.Eks, kfdManifest)
		oks = append(oks, cOks...)
		errs = append(errs, cErrs...)
	}

	etv := apis.NewExtraToolsValidatorFactory(tv.executor, miniConf.APIVersion, miniConf.Kind, tv.autoConnect)

	if etv == nil {
		return oks, errs
	}

	if xoks, xerrs := etv.Validate(tv.furyctlPath); len(xerrs) > 0 {
		errs = append(errs, xerrs...)
	} else {
		oks = append(oks, xoks...)
	}

	return oks, errs
}

func (tv *Validator) validateTools(i any, kfdManifest config.KFD) ([]string, []error) {
	var errs []error

	oks := make([]string, 0)

	toolCfgs := reflect.ValueOf(i)
	for i := range toolCfgs.NumField() {
		toolCfg, ok := toolCfgs.Field(i).Interface().(config.KFDTool)
		if !ok {
			continue
		}

		toolName := strings.ToLower(toolCfgs.Type().Field(i).Name)

		if (toolName == "helm" || toolName == "helmfile") &&
			!distribution.HasFeature(kfdManifest, distribution.FeaturePlugins) {
			continue
		}

		if (toolName == "yq") && !distribution.HasFeature(kfdManifest, distribution.FeatureYqSupport) {
			continue
		}

		tool := tv.toolFactory.Create(itool.Name(toolName), toolCfg.Version)
		if err := tool.CheckBinVersion(); err != nil {
			errs = append(errs, err)

			continue
		}

		oks = append(oks, toolName)
	}

	return oks, errs
}
