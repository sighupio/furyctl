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
	"github.com/sighupio/furyctl/internal/tool/kapp"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

func Test_Kapp_SupportsDownload(t *testing.T) {
	a := tools.NewKapp(newKappRunner(), "3.5.3")

	if a.SupportsDownload() != true {
		t.Errorf("kapp download must be supported")
	}
}

func Test_Kapp_SrcPath(t *testing.T) {
	wantSrcPath := fmt.Sprintf(
		"https://github.com/carvel-dev/kapp/releases/download/v3.5.3/kapp-%s-%s",
		runtime.GOOS,
		runtime.GOARCH,
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
			fa := tools.NewKapp(newKappRunner(), tC.version)
			if fa.SrcPath() != wantSrcPath {
				t.Errorf("Wrong kapp src path: want = %s, got = %s", wantSrcPath, fa.SrcPath())
			}
		})
	}
}

func Test_Kapp_Rename(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "furyctl-test-")
	if err != nil {
		t.Fatalf("error creating temp dir: %v", err)
	}

	binaryName := fmt.Sprintf("kapp-%s-%s", runtime.GOOS, runtime.GOARCH)

	if _, err := os.Create(filepath.Join(tmpDir, binaryName)); err != nil {
		t.Fatalf("error creating temp file: %v", err)
	}

	fa := tools.NewKapp(newKappRunner(), "3.5.3")

	if err := fa.Rename(tmpDir); err != nil {
		t.Fatalf("Error renaming kapp binary: %v", err)
	}

	info, err := os.Stat(filepath.Join(tmpDir, "kapp"))
	if err != nil {
		t.Fatalf("Error stating kapp binary: %v", err)
	}

	if info.IsDir() {
		t.Errorf("kapp binary is a directory")
	}
}

func Test_Kapp_CheckBinVersion(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc        string
		wantVersion string
		wantErrMsg  string
		wantErr     bool
	}{
		{
			desc:        "correct version installed",
			wantVersion: "0.62.0",
		},
		{
			desc:        "wrong version installed",
			wantVersion: "3.5.3",
			wantErr:     true,
			wantErrMsg:  "installed = 0.62.0, expected = 3.5.3",
		},
	}
	for _, tC := range testCases {
		tC := tC

		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			fa := tools.NewKapp(newKappRunner(), tC.wantVersion)

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

func newKappRunner() *kapp.Runner {
	return kapp.NewRunner(execx.NewFakeExecutor("TestHelperProcess"), kapp.Paths{
		Kapp: "kapp",
	}, false)
}
