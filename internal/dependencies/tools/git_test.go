// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package tools_test

import (
	"strings"
	"testing"

	"github.com/sighupio/furyctl/internal/dependencies/tools"
	"github.com/sighupio/furyctl/internal/tool/git"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

func Test_Git_SupportsDownload(t *testing.T) {
	a := tools.NewGit(newGitRunner(), "2.39.0")

	if a.SupportsDownload() != false {
		t.Errorf("Git download must not be supported")
	}
}

func Test_Git_CheckBinVersion(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc        string
		wantVersion string
		wantErr     bool
		wantErrMsg  string
	}{
		{
			desc:        "correct version installed",
			wantVersion: "2.39.0",
		},
		{
			desc:        "wrong version installed",
			wantVersion: "2.39.1",
			wantErr:     true,
			wantErrMsg:  "installed = 2.39.0, expected = 2.39.1",
		},
	}
	for _, tC := range testCases {
		tC := tC

		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			fa := tools.NewGit(newGitRunner(), tC.wantVersion)

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

func newGitRunner() *git.Runner {
	return git.NewRunner(execx.NewFakeExecutor(), git.Paths{
		Git: "git",
	})
}
