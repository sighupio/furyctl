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

func Test_Awscli_SrcPath(t *testing.T) {
	var wantSrcPath string

	if runtime.GOOS == "darwin" {
		wantSrcPath = "https://awscli.amazonaws.com/AWSCLIV2-0.3.0.pkg"
	}

	if runtime.GOOS == "linux" {
		wantSrcPath = "https://awscli.amazonaws.com/awscli-exe-linux-x84_64-0.3.0.zip"
	}

	testCases := []struct {
		desc    string
		version string
	}{
		{
			desc:    "0.3.0",
			version: "0.3.0",
		},
		{
			desc:    "v0.3.0",
			version: "v0.3.0",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			fa := tools.NewAwscli(newAwscliRunner(), tC.version)
			if fa.SrcPath() != wantSrcPath {
				t.Errorf("Wrong Awscli src path: want = %s, got = %s", wantSrcPath, fa.SrcPath())
			}
		})
	}
}

func Test_Awscli_Rename(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "furyctl-test-")
	if err != nil {
		t.Fatalf("error creating temp dir: %v", err)
	}

	if _, err := os.Create(filepath.Join(tmpDir, fmt.Sprintf("awscli-%s-x86_64", runtime.GOOS))); err != nil {
		t.Fatalf("error creating temp file: %v", err)
	}

	fa := tools.NewAwscli(newAwscliRunner(), "0.3.0")

	if err := fa.Rename(tmpDir); err != nil {
		t.Fatalf("Error renaming awscli binary: %v", err)
	}

	info, err := os.Stat(filepath.Join(tmpDir, "aws-cli"))
	if err != nil {
		t.Fatalf("Error stating awscli binary: %v", err)
	}

	if info.IsDir() {
		t.Errorf("awscli binary is a directory")
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
