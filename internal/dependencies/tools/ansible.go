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
		arch:    runtime.GOARCH,
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

// ansibleBundleBaseURL is the (PoC) location of the self-contained ansible-portable bundles.
// In production this becomes the SIGHUP-hosted release repo.
const ansibleBundleBaseURL = "https://github.com/nutellinoit/ansible-portable-poc/releases/download"

func (*Ansible) SupportsDownload() bool {
	return true
}

// SrcPath builds the bundle URL entirely from the configured version, which is the bundle's
// own release tag (e.g. "v0.2.0") used verbatim both as the release tag and in the filename.
func (a *Ansible) SrcPath() string {
	return fmt.Sprintf(
		"%s/%s/ansible-portable-%s-%s-%s.tar.gz",
		ansibleBundleBaseURL,
		a.version,
		a.version,
		a.os,
		a.arch,
	)
}

func (*Ansible) Rename(_ string) error {
	// The tarball already extracts to python/ + collections/ with executable bits preserved.
	return nil
}

// CheckBinVersion only verifies the bundle is present and runs: the configured version is the
// bundle release tag, not the ansible-core version reported by `ansible --version`, so a
// semver comparison would be meaningless.
func (a *Ansible) CheckBinVersion() error {
	if err := a.checker.presence(); err != nil {
		return fmt.Errorf("ansible: %w", err)
	}

	return nil
}
