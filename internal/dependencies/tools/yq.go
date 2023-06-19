// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/sighupio/furyctl/internal/semver"
	"github.com/sighupio/furyctl/internal/tool/yq"
)

func NewYq(runner *yq.Runner, version string) *Yq {
	return &Yq{
		arch:    runtime.GOARCH,
		os:      runtime.GOOS,
		version: version,
		checker: &checker{
			regex:  regexp.MustCompile(`yq \(https:\/\/github\.com\/mikefarah\/yq\/\) version .*`),
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

type Yq struct {
	arch    string
	checker *checker
	os      string
	version string
}

func (*Yq) SupportsDownload() bool {
	return true
}

func (y *Yq) SrcPath() string {
	return fmt.Sprintf(
		"https://github.com/mikefarah/yq/releases/download/%s/yq_%s_%s.tar.gz",
		semver.EnsurePrefix(y.version),
		y.os,
		y.arch,
	)
}

func (y *Yq) Rename(basePath string) error {
	oldName := fmt.Sprintf("yq_%s_%s", y.os, y.arch)
	newName := "yq"

	err := os.Rename(filepath.Join(basePath, oldName), filepath.Join(basePath, newName))
	if err != nil {
		return fmt.Errorf("error while renaming yq: %w", err)
	}

	return nil
}

func (y *Yq) CheckBinVersion() error {
	if err := y.checker.version(y.version); err != nil {
		return fmt.Errorf("yq: %w", err)
	}

	return nil
}
