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

	"github.com/sighupio/furyctl/internal/tool/awscli"
)

func NewAwscli(runner *awscli.Runner, version string) *Awscli {
	return &Awscli{
		arch:    "x86_64",
		os:      runtime.GOOS,
		version: version,
		checker: &checker{
			regex:  regexp.MustCompile(`aws-cli/(\S*)`),
			runner: runner,
			trimFn: func(tokens []string) string {
				return tokens[len(tokens)-1]
			},
			splitFn: func(version string) []string {
				return strings.Split(version, "/")
			},
		},
	}
}

type Awscli struct {
	arch    string
	checker *checker
	os      string
	version string
}

func (*Awscli) SupportsDownload() bool {
	return false
}

func (*Awscli) SrcPath() string {
	// Not used for this tool because it's not downloaded.
	return ""
}

func (*Awscli) Rename(_ string) error {
	// Not used for this tool because it's not downloaded.
	return nil
}

func (a *Awscli) CheckBinVersion() error {
	if err := a.checker.version(a.version); err != nil {
		return fmt.Errorf("aws-cli: %w", err)
	}

	return nil
}
