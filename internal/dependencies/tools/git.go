// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tools

import (
	"fmt"
	"regexp"
	"runtime"
	"strings"

	"github.com/sighupio/furyctl/internal/tool/git"
)

func NewGit(runner *git.Runner, version string) *Git {
	return &Git{
		arch:    "amd64",
		os:      runtime.GOOS,
		version: version,
		checker: &checker{
			regex:  regexp.MustCompile(`git version .*`),
			runner: runner,
			splitFn: func(version string) []string {
				return strings.Split(version, " ")
			},
			trimFn: func(tokens []string) string {
				return tokens[len(tokens)-1]
			},
		},
	}
}

type Git struct {
	arch    string
	checker *checker
	os      string
	version string
}

func (*Git) SupportsDownload() bool {
	return false
}

func (*Git) SrcPath() string {
	return ""
}

func (*Git) Rename(_ string) error {
	return nil
}

func (g *Git) CheckBinVersion() error {
	if err := g.checker.version(g.version); err != nil {
		return fmt.Errorf("git: %w", err)
	}

	return nil
}

func (g *Git) CmdPath() string {
	return g.checker.runner.CmdPath()
}

func (g *Git) OS() string {
	return g.os
}

func (g *Git) Arch() string {
	return g.arch
}
