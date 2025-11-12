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
	"github.com/sighupio/furyctl/internal/tool/git"
	"github.com/sighupio/furyctl/internal/tool/golang"
	"github.com/sighupio/furyctl/internal/tool/helm"
	"github.com/sighupio/furyctl/internal/tool/helmfile"
	"github.com/sighupio/furyctl/internal/tool/kapp"
	"github.com/sighupio/furyctl/internal/tool/kubectl"
	"github.com/sighupio/furyctl/internal/tool/kustomize"
	"github.com/sighupio/furyctl/internal/tool/openvpn"
	"github.com/sighupio/furyctl/internal/tool/sed"
	"github.com/sighupio/furyctl/internal/tool/shell"
	"github.com/sighupio/furyctl/internal/tool/terraform"
	"github.com/sighupio/furyctl/internal/tool/yq"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

var (
	errRegexNil             = errors.New("regex cannot be nil")
	errVersionEmpty         = errors.New("version cannot be empty")
	errCannotParseWithRegex = errors.New("can't parse system tool version using regex")
	errCannotParse          = errors.New("can't parse system tool version")
	errMissingBin           = errors.New("missing binary from vendor folder")
	errGetVersion           = errors.New("can't get tool version")
	ErrInvalidRunnerType    = errors.New("invalid runner type")
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

func (f *Factory) Create(name tool.Name, version string) Tool {
	t := f.runnerFactory.Create(name, version, "")

	toolMap := make(map[tool.Name]func() (Tool, error))

	toolMap[tool.Ansible] = func() (Tool, error) {
		a, ok := t.(*ansible.Runner)
		if !ok {
			return nil, fmt.Errorf("expected ansible.Runner, got %T: %w", t, ErrInvalidRunnerType)
		}

		return NewAnsible(a, version), nil
	}

	toolMap[tool.Awscli] = func() (Tool, error) {
		a, ok := t.(*awscli.Runner)
		if !ok {
			return nil, fmt.Errorf("expected awscli.Runner, got %T: %w", t, ErrInvalidRunnerType)
		}

		return NewAwscli(a, version), nil
	}

	toolMap[tool.Furyagent] = func() (Tool, error) {
		fa, ok := t.(*furyagent.Runner)
		if !ok {
			return nil, fmt.Errorf("expected furyagent.Runner, got %T: %w", t, ErrInvalidRunnerType)
		}

		return NewFuryagent(fa, version), nil
	}

	toolMap[tool.Git] = func() (Tool, error) {
		g, ok := t.(*git.Runner)
		if !ok {
			return nil, fmt.Errorf("expected git.Runner, got %T: %w", t, ErrInvalidRunnerType)
		}

		return NewGit(g, version), nil
	}

	toolMap[tool.Kubectl] = func() (Tool, error) {
		k, ok := t.(*kubectl.Runner)
		if !ok {
			return nil, fmt.Errorf("expected kubectl.Runner, got %T: %w", t, ErrInvalidRunnerType)
		}

		return NewKubectl(k, version), nil
	}

	toolMap[tool.Kustomize] = func() (Tool, error) {
		k, ok := t.(*kustomize.Runner)
		if !ok {
			return nil, fmt.Errorf("expected kustomize.Runner, got %T: %w", t, ErrInvalidRunnerType)
		}

		return NewKustomize(k, version), nil
	}

	toolMap[tool.Openvpn] = func() (Tool, error) {
		o, ok := t.(*openvpn.Runner)
		if !ok {
			return nil, fmt.Errorf("expected openvpn.Runner, got %T: %w", t, ErrInvalidRunnerType)
		}

		return NewOpenvpn(o, version), nil
	}

	toolMap[tool.Terraform] = func() (Tool, error) {
		tf, ok := t.(*terraform.Runner)
		if !ok {
			return nil, fmt.Errorf("expected terraform.Runner, got %T: %w", t, ErrInvalidRunnerType)
		}

		return NewTerraform(tf, version), nil
	}

	toolMap[tool.OpenTofu] = func() (Tool, error) {
		tofu, ok := t.(*terraform.Runner)
		if !ok {
			return nil, fmt.Errorf("expected terraform.Runner, got %T: %w", t, ErrInvalidRunnerType)
		}

		return NewOpenTofu(tofu, version), nil
	}

	toolMap[tool.Yq] = func() (Tool, error) {
		yqr, ok := t.(*yq.Runner)
		if !ok {
			return nil, fmt.Errorf("expected yq.Runner, got %T: %w", t, ErrInvalidRunnerType)
		}

		return NewYq(yqr, version), nil
	}

	toolMap[tool.Shell] = func() (Tool, error) {
		shellr, ok := t.(*shell.Runner)
		if !ok {
			return nil, fmt.Errorf("expected shell.Runner, got %T: %w", t, ErrInvalidRunnerType)
		}

		return NewShell(shellr, version), nil
	}

	toolMap[tool.Sed] = func() (Tool, error) {
		sedr, ok := t.(*sed.Runner)
		if !ok {
			return nil, fmt.Errorf("expected sed.Runner, got %T: %w", t, ErrInvalidRunnerType)
		}

		return NewSed(sedr, version), nil
	}

	toolMap[tool.Helm] = func() (Tool, error) {
		hr, ok := t.(*helm.Runner)
		if !ok {
			return nil, fmt.Errorf("expected helm.Runner, got %T: %w", t, ErrInvalidRunnerType)
		}

		return NewHelm(hr, version), nil
	}

	toolMap[tool.Helmfile] = func() (Tool, error) {
		hfr, ok := t.(*helmfile.Runner)
		if !ok {
			return nil, fmt.Errorf("expected helmfile.Runner, got %T: %w", t, ErrInvalidRunnerType)
		}

		return NewHelmfile(hfr, version), nil
	}

	toolMap[tool.Kapp] = func() (Tool, error) {
		ka, ok := t.(*kapp.Runner)
		if !ok {
			return nil, fmt.Errorf("expected kapp.Runner, got %T: %w", t, ErrInvalidRunnerType)
		}

		return NewKapp(ka, version), nil
	}

	toolMap[tool.Go] = func() (Tool, error) {
		golang, ok := t.(*golang.Runner)
		if !ok {
			return nil, fmt.Errorf("expected kapp.Runner, got %T: %w", t, ErrInvalidRunnerType)
		}

		return NewGolang(golang, version), nil
	}

	if createFunc, ok := toolMap[name]; ok {
		createdTool, err := createFunc()
		if err != nil {
			panic(err)
		}

		return createdTool
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

	sysVer, err := semver.NewVersion(systemVersion)
	if err != nil {
		return fmt.Errorf("%w: %v", errGetVersion, err)
	}

	wantVer, err := semver.NewConstraint(want)
	if err != nil {
		return fmt.Errorf("%w: %v", errGetVersion, err)
	}

	if !wantVer.Check(sysVer) {
		return fmt.Errorf("%w - installed = %s, expected = %s", ErrWrongToolVersion, systemVersion, want)
	}

	return nil
}
