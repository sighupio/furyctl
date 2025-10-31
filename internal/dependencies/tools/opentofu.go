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

func NewOpenTofu(runner *terraform.Runner, version string) *OpenTofu {
	return &OpenTofu{
		arch:    runtime.GOARCH,
		os:      runtime.GOOS,
		version: version,
		checker: &checker{
			regex:  regexp.MustCompile("OpenTofu .*"),
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

type OpenTofu struct {
	arch    string
	checker *checker
	os      string
	version string
}

func (*OpenTofu) SupportsDownload() bool {
	return true
}

func (t *OpenTofu) SrcPath() string {
	return fmt.Sprintf(
		"https://github.com/opentofu/opentofu/releases/download/v%s/tofu_%s_%s_%s.zip",
		semver.EnsureNoPrefix(t.version),
		semver.EnsureNoPrefix(t.version),
		t.os,
		t.arch,
	)
}

func (*OpenTofu) Rename(_ string) error {
	return nil
}

func (t *OpenTofu) CheckBinVersion() error {
	if err := t.checker.version(t.version); err != nil {
		return fmt.Errorf("opentofu: %w", err)
	}

	return nil
}
