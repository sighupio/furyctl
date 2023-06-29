// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package tools_test

import (
	"testing"

	"github.com/sighupio/furyctl/internal/dependencies/tools"
	"github.com/sighupio/furyctl/internal/tool/shell"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

func Test_Shell_SupportsDownload(t *testing.T) {
	a := tools.NewShell(newShellRunner(), "3.2.57")

	if a.SupportsDownload() != false {
		t.Errorf("shell download must not be supported")
	}
}

func newShellRunner() *shell.Runner {
	return shell.NewRunner(execx.NewFakeExecutor("TestHelperProcess"), shell.Paths{
		Shell: "sh",
	})
}
