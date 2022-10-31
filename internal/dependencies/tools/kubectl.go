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
	"github.com/sighupio/furyctl/internal/tool/kubectl"
)

func NewKubectl(runner *kubectl.Runner, version string) *Kubectl {
	return &Kubectl{
		arch:    "amd64",
		os:      runtime.GOOS,
		version: version,
		checker: &checker{
			regex:  regexp.MustCompile("GitVersion:\"([^\"]*)\""),
			runner: runner,
			trimFn: func(tokens []string) string {
				return strings.TrimRight(
					strings.TrimLeft(tokens[len(tokens)-1], "\"v"),
					"\"",
				)
			},
			splitFn: func(version string) []string {
				return strings.Split(version, ":")
			},
		},
	}
}

type Kubectl struct {
	arch    string
	checker *checker
	os      string
	version string
}

func (*Kubectl) SupportsDownload() bool {
	return true
}

func (k *Kubectl) SrcPath() string {
	return fmt.Sprintf(
		"https://dl.k8s.io/release/%s/bin/%s/%s/kubectl",
		semver.EnsurePrefix(k.version),
		k.os,
		k.arch,
	)
}

func (*Kubectl) Rename(_ string) error {
	return nil
}

func (k *Kubectl) CheckBinVersion() error {
	if err := k.checker.version(k.version); err != nil {
		return fmt.Errorf("kubectl: %w", err)
	}

	return nil
}
