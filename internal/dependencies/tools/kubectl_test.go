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
	"github.com/sighupio/furyctl/internal/tool/kubectl"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

func Test_Kubectl_SupportsDownload(t *testing.T) {
	a := tools.NewKubectl(newKubectlRunner(), "1.24.9")

	if a.SupportsDownload() != true {
		t.Errorf("kubectl download must be supported")
	}
}

func Test_Kubectl_SrcPath(t *testing.T) {
	wantSrcPath := fmt.Sprintf("https://dl.k8s.io/release/v1.24.9/bin/%s/amd64/kubectl", runtime.GOOS)

	testCases := []struct {
		desc    string
		version string
	}{
		{
			desc:    "1.24.9",
			version: "1.24.9",
		},
		{
			desc:    "v1.24.9",
			version: "v1.24.9",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			fa := tools.NewKubectl(newKubectlRunner(), tC.version)
			if fa.SrcPath() != wantSrcPath {
				t.Errorf("Wrong kubectl src path: want = %s, got = %s", wantSrcPath, fa.SrcPath())
			}
		})
	}
}

func Test_Kubectl_Rename(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "furyctl-test-")
	if err != nil {
		t.Fatalf("error creating temp dir: %v", err)
	}

	if _, err := os.Create(filepath.Join(tmpDir, "kubectl")); err != nil {
		t.Fatalf("error creating temp file: %v", err)
	}

	fa := tools.NewKubectl(newKubectlRunner(), "1.24.9")

	if err := fa.Rename(tmpDir); err != nil {
		t.Fatalf("Error renaming kubectl binary: %v", err)
	}

	info, err := os.Stat(filepath.Join(tmpDir, "kubectl"))
	if err != nil {
		t.Fatalf("Error stating kubectl binary: %v", err)
	}

	if info.IsDir() {
		t.Errorf("kubectl binary is a directory")
	}
}

func Test_Kubectl_CheckBinVersion(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc        string
		wantVersion string
		wantErr     bool
		wantErrMsg  string
	}{
		{
			desc:        "correct version installed",
			wantVersion: "1.21.1",
		},
		{
			desc:        "wrong version installed",
			wantVersion: "1.22.0",
			wantErr:     true,
			wantErrMsg:  "installed = 1.21.1, expected = 1.22.0",
		},
	}
	for _, tC := range testCases {
		tC := tC

		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			fa := tools.NewKubectl(newKubectlRunner(), tC.wantVersion)

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

func newKubectlRunner() *kubectl.Runner {
	return kubectl.NewRunner(execx.NewFakeExecutor(), kubectl.Paths{
		Kubectl: "kubectl",
	}, true, true)
}
