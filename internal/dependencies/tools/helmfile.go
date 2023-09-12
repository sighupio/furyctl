// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tools

import (
	"fmt"
	"runtime"

	"github.com/sighupio/furyctl/internal/semver"
	"github.com/sighupio/furyctl/internal/tool/helmfile"
)

func NewHelmfile(runner *helmfile.Runner, version string) *Helmfile {
	return &Helmfile{
		arch:    runtime.GOARCH,
		os:      runtime.GOOS,
		version: version,
		checker: &checker{
			runner: runner,
		},
	}
}

type Helmfile struct {
	arch    string
	checker *checker
	os      string
	version string
}

func (*Helmfile) SupportsDownload() bool {
	return true
}

func (h *Helmfile) SrcPath() string {
	return fmt.Sprintf(
		"https://github.com/helmfile/helmfile/releases/download/%s/helmfile_%s_%s_%s.tar.gz",
		semver.EnsurePrefix(h.version),
		h.version,
		h.os,
		h.arch,
	)
}

func (*Helmfile) Rename(_ string) error {
	return nil
}

func (h *Helmfile) CheckBinVersion() error {
	if err := h.checker.version(h.version); err != nil {
		return fmt.Errorf("helmfile: %w", err)
	}

	return nil
}
