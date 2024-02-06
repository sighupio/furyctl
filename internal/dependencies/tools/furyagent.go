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
	"github.com/sighupio/furyctl/internal/tool/furyagent"
	iox "github.com/sighupio/furyctl/internal/x/io"
)

func NewFuryagent(runner *furyagent.Runner, version string) *Furyagent {
	return &Furyagent{
		arch:    runtime.GOARCH,
		os:      runtime.GOOS,
		version: version,
		checker: &checker{
			regex:  regexp.MustCompile(`version (\S*)`),
			runner: runner,
			trimFn: func(tokens []string) string {
				return tokens[len(tokens)-1]
			},
			splitFn: func(version string) []string {
				return strings.Split(version, " ")
			},
		},
	}
}

type Furyagent struct {
	arch    string
	checker *checker
	os      string
	version string
}

func (*Furyagent) SupportsDownload() bool {
	return true
}

func (f *Furyagent) SrcPath() string {
	return fmt.Sprintf(
		"https://github.com/sighupio/furyagent/releases/download/%s/furyagent-%s-%s",
		semver.EnsurePrefix(f.version),
		f.os,
		f.arch,
	)
}

func (f *Furyagent) Rename(basePath string) error {
	oldPath := filepath.Join(basePath, fmt.Sprintf("furyagent-%s-%s", f.os, f.arch))
	newPath := filepath.Join(basePath, "furyagent")

	if err := iox.CopyFile(oldPath, newPath); err != nil {
		return fmt.Errorf("error while renaming furyagent: %w", err)
	}

	return nil
}

func (f *Furyagent) CheckBinVersion() error {
	if err := f.checker.version(f.version); err != nil {
		return fmt.Errorf("furyagent: %w", err)
	}

	return nil
}
