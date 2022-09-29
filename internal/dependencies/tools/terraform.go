// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tools

import (
	"fmt"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/sighupio/furyctl/internal/execx"
	"github.com/sighupio/furyctl/internal/semver"
)

const terraformVersionRegexString = "Terraform .*"

var terraformVersionRegex = regexp.MustCompile(terraformVersionRegexString)

func NewTerraform(version string) *Terraform {
	return &Terraform{
		executor: execx.NewStdExecutor(),
		version:  version,
		os:       runtime.GOOS,
		arch:     "amd64",
	}
}

type Terraform struct {
	executor execx.Executor
	version  string
	os       string
	arch     string
}

func (t *Terraform) SetExecutor(executor execx.Executor) {
	t.executor = executor
}

func (t *Terraform) SupportsDownload() bool {
	return true
}

func (t *Terraform) SrcPath() string {
	return fmt.Sprintf(
		"https://releases.hashicorp.com/terraform/%s/terraform_%s_%s_%s.zip",
		semver.EnsureNoPrefix(t.version, "v"),
		semver.EnsureNoPrefix(t.version, "v"),
		t.os,
		t.arch,
	)
}

func (t *Terraform) Rename(basePath string) error {
	return nil
}

func (t *Terraform) CheckBinVersion(binPath string) error {
	if t.version == "" {
		return fmt.Errorf("terraform: %w", ErrEmptyToolVersion)
	}

	path := filepath.Join(binPath, "terraform")
	out, err := t.executor.Command(path, "--version").Output()
	if err != nil {
		return fmt.Errorf("error running %s: %w", path, err)
	}

	s := string(out)

	versionStringIndex := terraformVersionRegex.FindStringIndex(s)
	if versionStringIndex == nil {
		return fmt.Errorf("can't get terraform version from system")
	}

	versionString := s[versionStringIndex[0]:versionStringIndex[1]]

	versionStringTokens := strings.Split(versionString, " ")
	if len(versionStringTokens) == 0 {
		return fmt.Errorf("can't get terraform version from system")
	}

	systemTerraformVersion := strings.TrimLeft(versionStringTokens[len(versionStringTokens)-1], "v")

	if systemTerraformVersion != t.version {
		return fmt.Errorf("%w: installed = %s, expected = %s", ErrWrongToolVersion, systemTerraformVersion, t.version)
	}

	return nil
}
