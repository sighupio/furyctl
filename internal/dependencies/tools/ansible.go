// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tools

import (
	"fmt"
	"regexp"
	"runtime"
	"strings"

	"github.com/sighupio/furyctl/internal/tool/ansible"
)

func NewAnsible(runner *ansible.Runner, version string) *Ansible {
	return &Ansible{
		arch:    "amd64",
		os:      runtime.GOOS,
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

type Ansible struct {
	arch    string
	checker *checker
	os      string
	version string
}

func (*Ansible) SupportsDownload() bool {
	return false
}

func (*Ansible) SrcPath() string {
	return ""
}

func (*Ansible) Rename(_ string) error {
	return nil
}

func (a *Ansible) CheckBinVersion() error {
	if err := a.checker.version(a.version); err != nil {
		return fmt.Errorf("ansible: %w", errMissingBin)
	}

	return nil
}
