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
	"github.com/sighupio/furyctl/internal/tool/kustomize"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

func Test_Kustomize_SupportsDownload(t *testing.T) {
	a := tools.NewKustomize(newKustomizeRunner(), "3.5.3")

	if a.SupportsDownload() != true {
		t.Errorf("kustomize download must be supported")
	}
}

func Test_Kustomize_SrcPath(t *testing.T) {
	wantSrcPath := fmt.Sprintf(
		"https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize/v3.5.3/kustomize_v3.5.3_%s_amd64.tar.gz",
		runtime.GOOS,
	)

	testCases := []struct {
		desc    string
		version string
	}{
		{
			desc:    "3.5.3",
			version: "3.5.3",
		},
		{
			desc:    "v3.5.3",
			version: "v3.5.3",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			fa := tools.NewKustomize(newKustomizeRunner(), tC.version)
			if fa.SrcPath() != wantSrcPath {
				t.Errorf("Wrong kustomize src path: want = %s, got = %s", wantSrcPath, fa.SrcPath())
			}
		})
	}
}

func Test_Kustomize_Rename(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "furyctl-test-")
	if err != nil {
		t.Fatalf("error creating temp dir: %v", err)
	}

	if _, err := os.Create(filepath.Join(tmpDir, "kustomize")); err != nil {
		t.Fatalf("error creating temp file: %v", err)
	}

	fa := tools.NewKustomize(newKustomizeRunner(), "3.5.3")

	if err := fa.Rename(tmpDir); err != nil {
		t.Fatalf("Error renaming kustomize binary: %v", err)
	}

	info, err := os.Stat(filepath.Join(tmpDir, "kustomize"))
	if err != nil {
		t.Fatalf("Error stating kustomize binary: %v", err)
	}

	if info.IsDir() {
		t.Errorf("kustomize binary is a directory")
	}
}

func Test_Kustomize_CheckBinVersion(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc        string
		wantVersion string
		wantErr     bool
		wantErrMsg  string
	}{
		{
			desc:        "correct version installed",
			wantVersion: "3.9.4",
		},
		{
			desc:        "wrong version installed",
			wantVersion: "3.5.3",
			wantErr:     true,
			wantErrMsg:  "installed = 3.9.4, expected = 3.5.3",
		},
	}
	for _, tC := range testCases {
		tC := tC

		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			fa := tools.NewKustomize(newKustomizeRunner(), tC.wantVersion)

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

func newKustomizeRunner() *kustomize.Runner {
	return kustomize.NewRunner(execx.NewFakeExecutor(), kustomize.Paths{
		Kustomize: "kustomize",
	})
}
