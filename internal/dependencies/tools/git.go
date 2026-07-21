// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tools

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/sighupio/furyctl/internal/semver"
	"github.com/sighupio/furyctl/internal/tool/git"
)

type Git struct {
	checker *checker
	version string
}

func NewGit(runner *git.Runner, version string) *Git {
	return &Git{
		version: version,
		checker: &checker{
			regex:  regexp.MustCompile("git version " + semver.Regex),
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

func (a *Git) CheckBinVersion() error {
	if err := a.checker.version(a.version); err != nil {
		return fmt.Errorf("git: %w", err)
	}

	return nil
}
