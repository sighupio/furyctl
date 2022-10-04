// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tools

import (
	"fmt"
	"regexp"

	"github.com/sighupio/furyctl/internal/tool"
	"github.com/sighupio/furyctl/internal/tool/ansible"
	"github.com/sighupio/furyctl/internal/tool/furyagent"
	"github.com/sighupio/furyctl/internal/tool/kubectl"
	"github.com/sighupio/furyctl/internal/tool/kustomize"
	"github.com/sighupio/furyctl/internal/tool/openvpn"
	"github.com/sighupio/furyctl/internal/tool/terraform"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

type Tool interface {
	SrcPath() string
	Rename(basePath string) error
	CheckBinVersion() error
	SupportsDownload() bool
}

func NewFactory(executor execx.Executor, paths FactoryPaths) *Factory {
	return &Factory{
		executor: executor,
		paths:    paths,
		runnerFactory: tool.NewRunnerFactory(executor, tool.RunnerFactoryPaths{
			Bin: paths.Bin,
		}),
	}
}

type FactoryPaths struct {
	Bin string
}

type Factory struct {
	executor      execx.Executor
	paths         FactoryPaths
	runnerFactory *tool.RunnerFactory
}

func (f *Factory) Create(name, version string) Tool {
	t := f.runnerFactory.Create(name, "")

	if name == tool.Ansible {
		return NewAnsible(t.(*ansible.Runner), version)
	}
	if name == tool.Furyagent {
		return NewFuryagent(t.(*furyagent.Runner), version)
	}
	if name == tool.Kubectl {
		return NewKubectl(t.(*kubectl.Runner), version)
	}
	if name == tool.Kustomize {
		return NewKustomize(t.(*kustomize.Runner), version)
	}
	if name == tool.Openvpn {
		return NewOpenvpn(t.(*openvpn.Runner), version)
	}
	if name == tool.Terraform {
		return NewTerraform(t.(*terraform.Runner), version)
	}
	return nil
}

type checker struct {
	regex   *regexp.Regexp
	runner  tool.Runner
	splitFn func(string) []string
	trimFn  func([]string) string
}

func (vc *checker) version(want string) error {
	if vc.regex == nil {
		return fmt.Errorf("regex cannot be nil")
	}

	if want == "" {
		return fmt.Errorf("version cannot be empty")
	}

	installed, err := vc.runner.Version()
	if err != nil {
		return fmt.Errorf("error getting version: %w", err)
	}

	versionStringIndex := vc.regex.FindStringIndex(installed)
	if versionStringIndex == nil {
		return fmt.Errorf("can't parse system tool version using regex '%s'", vc.regex.String())
	}

	versionString := installed[versionStringIndex[0]:versionStringIndex[1]]

	versionStringTokens := vc.splitFn(versionString)
	if len(versionStringTokens) == 0 {
		return fmt.Errorf("can't parse system tool version")
	}

	systemVersion := vc.trimFn(versionStringTokens)

	if systemVersion != want {
		return fmt.Errorf("%w - installed = %s, expected = %s", ErrWrongToolVersion, systemVersion, want)
	}

	return nil
}
