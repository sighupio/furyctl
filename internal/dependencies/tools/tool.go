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
		a, ok := t.(*ansible.Runner)
		if !ok {
			panic(fmt.Sprintf("expected ansible.Runner, got %T", t))
		}

		return NewAnsible(a, version)
	}
	
	if name == tool.Furyagent {
		fa, ok := t.(*furyagent.Runner)
		if !ok {
			panic(fmt.Sprintf("expected furyagent.Runner, got %T", t))
		}

		return NewFuryagent(fa, version)
	}
	
	if name == tool.Kubectl {
		k, ok := t.(*kubectl.Runner)
		if !ok {
			panic(fmt.Sprintf("expected kubectl.Runner, got %T", t))
		}

		return NewKubectl(k, version)
	}
	
	if name == tool.Kustomize {
		k, ok := t.(*kustomize.Runner)
		if !ok {
			panic(fmt.Sprintf("expected kustomize.Runner, got %T", t))
		}

		return NewKustomize(k, version)
	}
	
	if name == tool.Openvpn {
		o, ok := t.(*openvpn.Runner)
		if !ok {
			panic(fmt.Sprintf("expected openvpn.Runner, got %T", t))
		}

		return NewOpenvpn(o, version)
	}
	
	if name == tool.Terraform {
		tf, ok := t.(*terraform.Runner)
		if !ok {
			panic(fmt.Sprintf("expected terraform.Runner, got %T", t))
		}

		return NewTerraform(tf, version)
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
