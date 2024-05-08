// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tools

import (
	"fmt"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/sighupio/furyctl/internal/semver"
	"github.com/sighupio/furyctl/internal/tool/kapp"
	iox "github.com/sighupio/furyctl/internal/x/io"
)

func NewKapp(runner *kapp.Runner, version string) *Kapp {
	return &Kapp{
		arch:    runtime.GOARCH,
		os:      runtime.GOOS,
		version: version,
		checker: &checker{
			regex:  regexp.MustCompile(fmt.Sprintf("kapp version %s", semver.Regex)),
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

type Kapp struct {
	arch    string
	checker *checker
	os      string
	version string
}

func (*Kapp) SupportsDownload() bool {
	return true
}

func (k *Kapp) SrcPath() string {
	return fmt.Sprintf(
		"https://github.com/carvel-dev/kapp/releases/download/%s/kapp-%s-%s",
		semver.EnsurePrefix(k.version),
		k.os,
		k.arch,
	)
}

func (k *Kapp) Rename(basePath string) error {
	oldPath := filepath.Join(basePath, fmt.Sprintf("kapp-%s-%s", k.os, k.arch))
	newPath := filepath.Join(basePath, "kapp")

	if err := iox.CopyFile(oldPath, newPath); err != nil {
		return fmt.Errorf("error while renaming kapp: %w", err)
	}

	return nil
}

func (k *Kapp) CheckBinVersion() error {
	if err := k.checker.version(k.version); err != nil {
		return fmt.Errorf("kapp: %w", err)
	}

	return nil
}
