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

func (f *Awscli) SrcPath() string {
	if f.os == "darwin" {
		return fmt.Sprintf(
			"https://awscli.amazonaws.com/AWSCLIV2-%s.pkg",
			semver.EnsureNoPrefix(f.version),
		)
	}

	if f.os == "linux" {
		return fmt.Sprintf(
			"https://awscli.amazonaws.com/awscli-exe-linux-%s-%s-%s.zip",
			semver.EnsurePrefix(f.version),
			f.os,
			f.arch,
		)
	}

	return ""
}

func (f *Awscli) Rename(basePath string) error {
	oldName := fmt.Sprintf("awscli-%s-%s", f.os, f.arch)
	newName := "aws-cli"

	err := os.Rename(filepath.Join(basePath, oldName), filepath.Join(basePath, newName))
	if err != nil {
		return fmt.Errorf("error while renaming aws-cli: %w", err)
	}

	return nil
}

func (f *Awscli) CheckBinVersion() error {
	if err := f.checker.version(f.version); err != nil {
		return fmt.Errorf("aws-cli: %w", err)
	}

	return nil
}

func getSrcPath(os, arch, version string) string {
	if os == "darwin" {
		return fmt.Sprintf(
			"https://awscli.amazonaws.com/AWSCLIV2-%s-%s.pkg",
			semver.EnsurePrefix(version),
			arch,
		)
	}

	if os == "linux" {
		return fmt.Sprintf(
			"https://awscli.amazonaws.com/awscli-exe-linux-%s-%s.zip",
			semver.EnsurePrefix(version),
			arch,
		)
	}

	return ""
}
