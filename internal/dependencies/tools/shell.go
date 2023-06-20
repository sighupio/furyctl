// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tools

import (
	"runtime"

	"github.com/sighupio/furyctl/internal/tool/shell"
)

func NewShell(runner *shell.Runner, version string) *Shell {
	return &Shell{
		arch:    runtime.GOARCH,
		os:      runtime.GOOS,
		version: version,
	}
}

type Shell struct {
	arch    string
	checker *checker
	os      string
	version string
}

func (*Shell) SupportsDownload() bool {
	return false
}

func (*Shell) SrcPath() string {
	return ""
}

func (*Shell) Rename(_ string) error {
	return nil
}

func (a *Shell) CheckBinVersion() error {
	return nil
}
