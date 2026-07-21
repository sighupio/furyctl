// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tools

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/sighupio/furyctl/internal/tool/awscli"
)

type Awscli struct {
	checker *checker
	version string
}

func NewAwscli(runner *awscli.Runner, version string) *Awscli {
	return &Awscli{
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

func (a *Awscli) CheckBinVersion() error {
	if err := a.checker.version(a.version); err != nil {
		return fmt.Errorf("awscli: %w", err)
	}

	return nil
}
