// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tools

import (
	"fmt"
	"regexp"
	"runtime"
	"strings"

	"github.com/sighupio/furyctl/internal/semver"
	"github.com/sighupio/furyctl/internal/tool/terraform"
)

// hasTerraformDarwinArm64Support should be dropped once furyctl removes support for fury v1.25.
func hasTerraformDarwinArm64Support(version string) bool {
	v, err := semver.NewVersion(version)
	if err != nil {
		return false
	}

	v102, err := semver.NewVersion("v1.0.2")
	if err != nil {
		return false
	}

	return v.GreaterThanOrEqual(v102)
}

func NewTerraform(runner *terraform.Runner, version string) *Terraform {
	arch := runtime.GOARCH

	if !hasTerraformDarwinArm64Support(version) {
		arch = "amd64"
	}

	return &Terraform{
		arch:    arch,
		os:      runtime.GOOS,
		version: version,
		checker: &checker{
			regex:  regexp.MustCompile("Terraform .*"),
			runner: runner,
			trimFn: func(tokens []string) string {
				return strings.TrimLeft(tokens[len(tokens)-1], "v")
			},
			splitFn: func(version string) []string {
				return strings.Split(version, " ")
			},
		},
	}
}

type Terraform struct {
	arch    string
	checker *checker
	os      string
	version string
}

func (*Terraform) SupportsDownload() bool {
	return true
}

func (t *Terraform) SrcPath() string {
	return fmt.Sprintf(
		"https://releases.hashicorp.com/terraform/%s/terraform_%s_%s_%s.zip",
		semver.EnsureNoPrefix(t.version),
		semver.EnsureNoPrefix(t.version),
		t.os,
		t.arch,
	)
}

func (*Terraform) Rename(_ string) error {
	return nil
}

func (t *Terraform) CheckBinVersion() error {
	if err := t.checker.version(t.version); err != nil {
		return fmt.Errorf("terraform: %w", err)
	}

	return nil
}
