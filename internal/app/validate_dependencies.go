// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/execx"
	"github.com/sighupio/furyctl/internal/netx"
)

var (
	ErrEmptyToolVersion = errors.New("empty tool version")
	ErrMissingEnvVar    = errors.New("missing environment variable")
	ErrWrongToolVersion = errors.New("wrong tool version")
)

type ValidateDependenciesRequest struct {
	BinPath           string
	FuryctlBinVersion string
	DistroLocation    string
	FuryctlConfPath   string
	Debug             bool
}

type ValidateDependenciesResponse struct {
	Errors   []error
	RepoPath string
}

func (vdr *ValidateDependenciesResponse) appendErrors(errs []error) {
	vdr.Errors = append(vdr.Errors, errs...)
}

func (vdr *ValidateDependenciesResponse) HasErrors() bool {
	return len(vdr.Errors) > 0
}

func NewValidateDependencies(client netx.Client, executor execx.Executor) *ValidateDependencies {
	return &ValidateDependencies{
		client:   client,
		executor: executor,
	}
}

type ValidateDependencies struct {
	client   netx.Client
	executor execx.Executor
}

func (vd *ValidateDependencies) Execute(req ValidateDependenciesRequest) (ValidateDependenciesResponse, error) {
	dloader := distribution.NewDownloader(vd.client, req.Debug)

	res := ValidateDependenciesResponse{}

	dres, err := dloader.Download(req.FuryctlBinVersion, req.DistroLocation, req.FuryctlConfPath)
	if err != nil {
		return res, err
	}

	res.RepoPath = dres.RepoPath
	res.appendErrors(vd.validateSystemDependencies(dres.DistroManifest, req.BinPath))
	res.appendErrors(vd.validateEnvVarsDependencies(dres.MinimalConf.Kind))

	return res, nil
}

func (vd *ValidateDependencies) validateEnvVarsDependencies(kind distribution.Kind) []error {
	errs := make([]error, 0)

	if kind.Equals(distribution.EKSCluster) {
		if os.Getenv("AWS_ACCESS_KEY_ID") == "" {
			errs = append(errs, fmt.Errorf("%w: AWS_ACCESS_KEY_ID", ErrMissingEnvVar))
		}

		if os.Getenv("AWS_SECRET_ACCESS_KEY") == "" {
			errs = append(errs, fmt.Errorf("%w: AWS_SECRET_ACCESS_KEY", ErrMissingEnvVar))
		}

		if os.Getenv("AWS_DEFAULT_REGION") == "" {
			errs = append(errs, fmt.Errorf("%w: AWS_DEFAULT_REGION", ErrMissingEnvVar))
		}
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}

func (vd *ValidateDependencies) validateSystemDependencies(kfdManifest distribution.Manifest, binPath string) []error {
	errs := make([]error, 0)

	// binPath is empty here because ansible is not handled by vendor dependencies
	if err := vd.checkAnsibleVersion(kfdManifest.Tools.Ansible, ""); err != nil {
		errs = append(errs, err)
	}

	if err := vd.checkTerraformVersion(kfdManifest.Tools.Terraform, binPath); err != nil {
		errs = append(errs, err)
	}

	if err := vd.checkKubectlVersion(kfdManifest.Tools.Kubectl, binPath); err != nil {
		errs = append(errs, err)
	}

	if err := vd.checkKustomizeVersion(kfdManifest.Tools.Kustomize, binPath); err != nil {
		errs = append(errs, err)
	}

	if kfdManifest.Tools.Furyagent != "" {
		if err := vd.checkFuryagentVersion(kfdManifest.Tools.Furyagent, binPath); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}

func (vd *ValidateDependencies) checkAnsibleVersion(wantVer, binPath string) error {
	if wantVer == "" {
		return fmt.Errorf("ansible: %w", ErrEmptyToolVersion)
	}

	path := filepath.Join(binPath, "ansible")
	out, err := vd.executor.Command(path, "--version").Output()
	if err != nil {
		return err
	}

	s := string(out)

	pattern := regexp.MustCompile("ansible \\[.*]")

	versionStringIndex := pattern.FindStringIndex(s)
	if versionStringIndex == nil {
		return fmt.Errorf("can't get ansible version from system")
	}

	versionString := s[versionStringIndex[0]:versionStringIndex[1]]

	versionStringTokens := strings.Split(versionString, " ")
	if len(versionStringTokens) == 0 {
		return fmt.Errorf("can't get ansible version from system")
	}

	systemAnsibleVersion := strings.TrimRight(versionStringTokens[len(versionStringTokens)-1], "]")

	if systemAnsibleVersion != wantVer {
		return fmt.Errorf("%w: installed = %s, expected = %s", ErrWrongToolVersion, systemAnsibleVersion, wantVer)
	}

	return nil
}

func (vd *ValidateDependencies) checkTerraformVersion(wantVer, binPath string) error {
	if wantVer == "" {
		return fmt.Errorf("terraform: %w", ErrEmptyToolVersion)
	}

	path := filepath.Join(binPath, "terraform")
	out, err := vd.executor.Command(path, "--version").Output()
	if err != nil {
		return err
	}

	s := string(out)

	pattern := regexp.MustCompile("Terraform .*")

	versionStringIndex := pattern.FindStringIndex(s)
	if versionStringIndex == nil {
		return fmt.Errorf("can't get terraform version from system")
	}

	versionString := s[versionStringIndex[0]:versionStringIndex[1]]

	versionStringTokens := strings.Split(versionString, " ")
	if len(versionStringTokens) == 0 {
		return fmt.Errorf("can't get terraform version from system")
	}

	systemTerraformVersion := strings.TrimLeft(versionStringTokens[len(versionStringTokens)-1], "v")

	if systemTerraformVersion != wantVer {
		return fmt.Errorf("%w: installed = %s, expected = %s", ErrWrongToolVersion, systemTerraformVersion, wantVer)
	}

	return nil
}

func (vd *ValidateDependencies) checkKubectlVersion(wantVer, binPath string) error {
	if wantVer == "" {
		return fmt.Errorf("kubectl: %w", ErrEmptyToolVersion)
	}

	path := filepath.Join(binPath, "kubectl")
	out, err := vd.executor.Command(path, "version", "--client").Output()
	if err != nil {
		return err
	}

	s := string(out)

	pattern := regexp.MustCompile("GitVersion:\"([^\"]*)\"")

	versionStringIndex := pattern.FindStringIndex(s)
	if versionStringIndex == nil {
		return fmt.Errorf("can't get kubectl version from system")
	}

	versionString := s[versionStringIndex[0]:versionStringIndex[1]]

	versionStringTokens := strings.Split(versionString, ":")
	if len(versionStringTokens) == 0 {
		return fmt.Errorf("can't get kubectl version from system")
	}

	systemKubectlVersion := strings.TrimRight(
		strings.TrimLeft(versionStringTokens[len(versionStringTokens)-1], "\"v"),
		"\"",
	)

	if systemKubectlVersion != wantVer {
		return fmt.Errorf("%w: installed = %s, expected = %s", ErrWrongToolVersion, systemKubectlVersion, wantVer)
	}

	return nil
}

func (vd *ValidateDependencies) checkKustomizeVersion(wantVer, binPath string) error {
	if wantVer == "" {
		return fmt.Errorf("kustomize: %w", ErrEmptyToolVersion)
	}

	path := filepath.Join(binPath, "kustomize")
	out, err := vd.executor.Command(path, "version", "--short").Output()
	if err != nil {
		return err
	}

	s := string(out)

	pattern := regexp.MustCompile("kustomize/v(\\S*)")

	versionStringIndex := pattern.FindStringIndex(s)
	if versionStringIndex == nil {
		return fmt.Errorf("can't get kustomize version from system")
	}

	versionString := s[versionStringIndex[0]:versionStringIndex[1]]

	versionStringTokens := strings.Split(versionString, "/")
	if len(versionStringTokens) == 0 {
		return fmt.Errorf("can't get kustomize version from system")
	}

	systemKustomizeVersion := strings.TrimLeft(versionStringTokens[len(versionStringTokens)-1], "v")

	if systemKustomizeVersion != wantVer {
		return fmt.Errorf("%w: installed = %s, expected = %s", ErrWrongToolVersion, systemKustomizeVersion, wantVer)
	}

	return nil
}

func (vd *ValidateDependencies) checkFuryagentVersion(wantVer, binPath string) error {
	if wantVer == "" {
		return fmt.Errorf("furyagent: %w", ErrEmptyToolVersion)
	}

	path := filepath.Join(binPath, "furyagent")
	out, err := vd.executor.Command(path, "version").Output()
	if err != nil {
		return err
	}

	s := string(out)

	pattern := regexp.MustCompile("version (\\S*)")

	versionStringIndex := pattern.FindStringIndex(s)
	if versionStringIndex == nil {
		return fmt.Errorf("can't get furyagent version from system")
	}

	versionString := s[versionStringIndex[0]:versionStringIndex[1]]

	versionStringTokens := strings.Split(versionString, " ")
	if len(versionStringTokens) == 0 {
		return fmt.Errorf("can't get furyagent version from system")
	}

	systemFuryagentVersion := versionStringTokens[len(versionStringTokens)-1]

	if systemFuryagentVersion != wantVer {
		return fmt.Errorf("%w: installed = %s, expected = %s", ErrWrongToolVersion, systemFuryagentVersion, wantVer)
	}

	return nil
}
