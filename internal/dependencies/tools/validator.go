// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tools

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/sighupio/furyctl/internal/apis"
	"github.com/sighupio/furyctl/internal/apis/config"
	"github.com/sighupio/furyctl/internal/distribution"
	itool "github.com/sighupio/furyctl/internal/tool"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

var (
	ErrWrongToolVersion = errors.New("wrong tool version")
	ErrToolNotFound     = errors.New("tool not found in the toolFactory, possible fury-distribution version mismatch")
)

type Validator struct {
	executor    execx.Executor
	toolFactory *Factory
	binPath     string
	furyctlPath string
}

func NewValidator(executor execx.Executor, binPath, furyctlPath string) *Validator {
	return &Validator{
		executor: executor,
		toolFactory: NewFactory(executor, FactoryPaths{
			Bin: binPath,
		}),
		binPath:     binPath,
		furyctlPath: furyctlPath,
	}
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

	etv := apis.NewExtraToolsValidatorFactory(
		tv.executor,
		miniConf.APIVersion,
		miniConf.Kind,
		kfdManifest,
		tv.binPath,
	)

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
		toolCfg, ok := reflect.TypeAssert[config.KFDTool](toolCfgs.Field(i))
		if !ok {
			continue
		}

		// Skip tools without a pinned version: with the back-compatible union model a tool is
		// pinned in only one section, so the other section's field is empty.
		if toolCfg.Version == "" {
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

		if (toolName == "kapp") && !distribution.HasFeature(kfdManifest, distribution.FeatureKappSupport) {
			continue
		}

		tool := tv.toolFactory.Create(itool.Name(toolName), toolCfg.Version)

		if tool == nil {
			errs = append(
				errs,
				fmt.Errorf("%s version %s: %w", toolName, toolCfg.Version, ErrToolNotFound),
			)

			continue
		}

		if err := tool.CheckBinVersion(); err != nil {
			errs = append(errs, err)

			continue
		}

		oks = append(oks, toolName)
	}

	return oks, errs
}
