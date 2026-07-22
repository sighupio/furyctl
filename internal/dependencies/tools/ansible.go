// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tools

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/sighupio/furyctl/internal/tool/ansible"
)

type Ansible struct {
	checker *checker
	version string
}

func NewAnsible(runner *ansible.Runner, version string) *Ansible {
	return &Ansible{
		version: version,
		checker: &checker{
			regex:  regexp.MustCompile(`ansible \[.*]`),
			runner: runner,
			splitFn: func(version string) []string {
				return strings.Split(version, " ")
			},
			trimFn: func(tokens []string) string {
				return strings.TrimRight(tokens[len(tokens)-1], "]")
			},
		},
	}
}

func (a *Ansible) CheckBinVersion() error {
	if err := a.checker.version(a.version); err != nil {
		return fmt.Errorf("ansible: %w", err)
	}

	return nil
}
