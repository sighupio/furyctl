// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package tools_test

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/sighupio/furyctl/internal/dependencies/tools"
	"github.com/sighupio/furyctl/internal/tool/awscli"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

func Test_Awscli_SupportsDownload(t *testing.T) {
	a := tools.NewAwscli(newAwscliRunner(), "0.3.0")

	if a.SupportsDownload() != false {
		t.Errorf("Awscli download must not be supported")
	}
}

func Test_Awscli_CheckBinVersion(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc        string
		wantVersion string
		wantErr     bool
		wantErrMsg  string
	}{
		{
			desc:        "correct version installed",
			wantVersion: "2.8.12",
		},
		{
			desc:        "wrong version installed",
			wantVersion: "1.22.0",
			wantErr:     true,
			wantErrMsg:  "installed = 2.8.12, expected = 1.22.0",
		},
	}
	for _, tC := range testCases {
		tC := tC

		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			fa := tools.NewAwscli(newAwscliRunner(), tC.wantVersion)

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

func newAwscliRunner() *awscli.Runner {
	return awscli.NewRunner(execx.NewFakeExecutor(), awscli.Paths{
		Awscli: "aws",
	})
}
