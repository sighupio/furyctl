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
	"github.com/sighupio/furyctl/internal/tool/kustomize"
)

func NewKustomize(runner *kustomize.Runner, version string) *Kustomize {
	return &Kustomize{
		arch:    "amd64",
		os:      runtime.GOOS,
		version: version,
		checker: &checker{
			regex:  regexp.MustCompile(`kustomize/v(\S*)`),
			runner: runner,
			trimFn: func(tokens []string) string {
				return strings.TrimLeft(tokens[len(tokens)-1], "v")
			},
			splitFn: func(version string) []string {
				return strings.Split(version, "/")
			},
		},
	}
}

type Kustomize struct {
	arch    string
	checker *checker
	os      string
	version string
}

func (*Kustomize) SupportsDownload() bool {
	return true
}

func (k *Kustomize) SrcPath() string {
	return fmt.Sprintf(
		"https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize/%s/kustomize_%s_%s_%s.tar.gz",
		semver.EnsurePrefix(k.version),
		semver.EnsurePrefix(k.version),
		k.os,
		k.arch,
	)
}

func (*Kustomize) Rename(_ string) error {
	return nil
}

func (k *Kustomize) CheckBinVersion() error {
	if err := k.checker.version(k.version); err != nil {
		return fmt.Errorf("kustomize: %w", err)
	}

	return nil
}
