// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tools

import (
	"github.com/sighupio/furyctl/internal/tool/shell"
)

func NewShell(_ *shell.Runner, _ string) *Shell {
	return &Shell{}
}

type Shell struct{}

func (*Shell) CheckBinVersion() error {
	return nil
}
