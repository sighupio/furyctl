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
	"github.com/sighupio/furyctl/internal/tool/yq"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

func Test_Yq_SupportsDownload(t *testing.T) {
	yqr := tools.NewYq(newYqRunner(), "4.34.1")

	if yqr.SupportsDownload() != true {
		t.Errorf("yq download must be supported")
	}
}

func Test_Yq_SrcPath(t *testing.T) {
	wantSrcPath := fmt.Sprintf(
		"https://github.com/mikefarah/yq/releases/download/v4.34.1/yq_%s_%s.tar.gz",
		runtime.GOOS,
		runtime.GOARCH,
	)

	testCases := []struct {
		desc    string
		version string
	}{
		{
			desc:    "4.34.1",
			version: "4.34.1",
		},
		{
			desc:    "v4.34.1",
			version: "v4.34.1",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			yqr := tools.NewYq(newYqRunner(), tC.version)
			if yqr.SrcPath() != wantSrcPath {
				t.Errorf("Wrong yq src path: want = %s, got = %s", wantSrcPath, yqr.SrcPath())
			}
		})
	}
}

func Test_Yq_Rename(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "furyctl-test-")
	if err != nil {
		t.Fatalf("error creating temp dir: %v", err)
	}

	if _, err := os.Create(filepath.Join(tmpDir, fmt.Sprintf(
		"yq_%s_%s",
		runtime.GOOS,
		runtime.GOARCH,
	))); err != nil {
		t.Fatalf("error creating temp file: %v", err)
	}

	yqr := tools.NewYq(newYqRunner(), "4.34.1")

	if err := yqr.Rename(tmpDir); err != nil {
		t.Fatalf("Error renaming yq binary: %v", err)
	}

	info, err := os.Stat(filepath.Join(tmpDir, "yq"))
	if err != nil {
		t.Fatalf("Error stating yq binary: %v", err)
	}

	if info.IsDir() {
		t.Errorf("yq binary is a directory")
	}
}

func Test_Yq_CheckBinVersion(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc        string
		wantVersion string
		wantErr     bool
		wantErrMsg  string
	}{
		{
			desc:        "correct version installed",
			wantVersion: "4.34.1",
		},
		{
			desc:        "wrong version installed",
			wantVersion: "1.34.1",
			wantErr:     true,
			wantErrMsg:  "installed = 4.34.1, expected = 1.34.1",
		},
	}
	for _, tC := range testCases {
		tC := tC

		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			yqr := tools.NewYq(newYqRunner(), tC.wantVersion)

			err := yqr.CheckBinVersion()

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

func newYqRunner() *yq.Runner {
	return yq.NewRunner(execx.NewFakeExecutor(), yq.Paths{
		Yq: "yq",
	})
}
