// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tools

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/sighupio/furyctl/internal/semver"
	"github.com/sighupio/furyctl/internal/tool"
	"github.com/sighupio/furyctl/internal/tool/ansible"
	"github.com/sighupio/furyctl/internal/tool/awscli"
	"github.com/sighupio/furyctl/internal/tool/furyagent"
	"github.com/sighupio/furyctl/internal/tool/kubectl"
	"github.com/sighupio/furyctl/internal/tool/kustomize"
	"github.com/sighupio/furyctl/internal/tool/openvpn"
	"github.com/sighupio/furyctl/internal/tool/terraform"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

var (
	errRegexNil             = errors.New("regex cannot be nil")
	errVersionEmpty         = errors.New("version cannot be empty")
	errCannotParseWithRegex = errors.New("can't parse system tool version using regex")
	errCannotParse          = errors.New("can't parse system tool version")
	errMissingBin           = errors.New("missing binary from vendor folder")
	errGetVersion           = errors.New("can't get tool version")
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
	t := f.runnerFactory.Create(name, version, "")

	if name == tool.Ansible {
		a, ok := t.(*ansible.Runner)
		if !ok {
			panic(fmt.Sprintf("expected ansible.Runner, got %T", t))
		}

		return NewAnsible(a, version)
	}

	if name == tool.Awscli {
		a, ok := t.(*awscli.Runner)
		if !ok {
			panic(fmt.Sprintf("expected awscli.Runner, got %T", t))
		}

		return NewAwscli(a, version)
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
		return errRegexNil
	}

	if want == "" {
		return errVersionEmpty
	}

	cmdDir := filepath.Dir(vc.runner.CmdPath())

	if _, err := os.Stat(cmdDir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errMissingBin
		}

		return fmt.Errorf("%w: %v", errGetVersion, err)
	}

	installed, err := vc.runner.Version()
	if err != nil {
		return fmt.Errorf("%w: %v", errGetVersion, err)
	}

	versionStringIndex := vc.regex.FindStringIndex(installed)
	if versionStringIndex == nil {
		return fmt.Errorf("%w '%s'", errCannotParseWithRegex, vc.regex.String())
	}

	versionString := installed[versionStringIndex[0]:versionStringIndex[1]]

	versionStringTokens := vc.splitFn(versionString)
	if len(versionStringTokens) == 0 {
		return errCannotParse
	}

	systemVersion := vc.trimFn(versionStringTokens)
	sysVerParts := semver.Parts(systemVersion)
	wantVerParts := semver.Parts(want)

	if !wantVerParts.CheckCompatibility(sysVerParts) {
		return fmt.Errorf("%w - installed = %s, expected = %s", ErrWrongToolVersion, systemVersion, want)
	}

	return nil
}
