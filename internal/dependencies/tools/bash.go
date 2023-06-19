// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tools

import (
	"fmt"
	"regexp"
	"runtime"
	"strings"

	"github.com/sighupio/furyctl/internal/tool/bash"
)

func NewBash(runner *bash.Runner, version string) *Bash {
	return &Bash{
		arch:    runtime.GOARCH,
		os:      runtime.GOOS,
		version: version,
		checker: &checker{
			regex:  regexp.MustCompile(`version (\S*)\(`),
			runner: runner,
			splitFn: func(version string) []string {
				return strings.Split(version, " ")
			},
			trimFn: func(tokens []string) string {
				return strings.TrimSuffix(tokens[len(tokens)-1], "(")
			},
		},
	}
}

type Bash struct {
	arch    string
	checker *checker
	os      string
	version string
}

func (*Bash) SupportsDownload() bool {
	return false
}

func (*Bash) SrcPath() string {
	return ""
}

func (*Bash) Rename(_ string) error {
	return nil
}

func (a *Bash) CheckBinVersion() error {
	if err := a.checker.version(a.version); err != nil {
		return fmt.Errorf("bash: %w", err)
	}

	return nil
}
