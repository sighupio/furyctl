// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package tools_test

import (
	"strings"
	"testing"

	"github.com/sighupio/furyctl/internal/dependencies/tools"
	"github.com/sighupio/furyctl/internal/tool/bash"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

func Test_Bash_SupportsDownload(t *testing.T) {
	a := tools.NewBash(newBashRunner(), "3.2.57")

	if a.SupportsDownload() != false {
		t.Errorf("bash download must not be supported")
	}
}

func Test_Bash_CheckBinVersion(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc        string
		wantVersion string
		wantErr     bool
		wantErrMsg  string
	}{
		{
			desc:        "correct version installed",
			wantVersion: "3.2.57",
		},
		{
			desc:        "wrong version installed",
			wantVersion: "1.2.57",
			wantErr:     true,
			wantErrMsg:  "installed = 3.2.57, expected = 1.2.57",
		},
	}
	for _, tC := range testCases {
		tC := tC

		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			fa := tools.NewBash(newBashRunner(), tC.wantVersion)

			err := fa.CheckBinVersion()

			if tC.wantErr && err == nil {
				t.Errorf("expected error, got nil")
			}

			if !tC.wantErr && err != nil {
				t.Errorf("expected no error, got %v", err)
			}

			if tC.wantErr && err != nil && !strings.Contains(err.Error(), tC.wantErrMsg) {
				t.Errorf("expected error message '%s' to contain '%s'", err.Error(), tC.wantErrMsg)
			}
		})
	}
}

func newBashRunner() *bash.Runner {
	return bash.NewRunner(execx.NewFakeExecutor(), bash.Paths{
		Bash: "bash",
	})
}
