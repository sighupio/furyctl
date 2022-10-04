// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//nolint:dupl // false positive
package tools

import (
	"fmt"
	"regexp"
	"runtime"
	"strings"

	"github.com/sighupio/furyctl/internal/semver"
	"github.com/sighupio/furyctl/internal/tool/terraform"
)

func NewTerraform(runner *terraform.Runner, version string) *Terraform {
	return &Terraform{
		arch:    "amd64",
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

func (t *Terraform) CheckBinVersion() error {
	if err := t.checker.version(t.version); err != nil {
		return fmt.Errorf("terraform: %w", err)
	}

	return nil
}
