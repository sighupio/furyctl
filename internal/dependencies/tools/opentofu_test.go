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
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

func Test_OpenTofu_SupportsDownload(t *testing.T) {
	a := tools.NewOpenTofu(newOpenTofuRunner(), "1.10.0")

	if a.SupportsDownload() != true {
		t.Errorf("opentofu download must be supported")
	}
}

func Test_OpenTofu_SrcPath(t *testing.T) {
	wantSrcPath := fmt.Sprintf(
		"https://github.com/opentofu/opentofu/releases/download/v1.10.0/tofu_1.10.0_%s_%s.zip",
		runtime.GOOS,
		runtime.GOARCH,
	)

	testCases := []struct {
		desc    string
		version string
	}{
		{
			desc:    "1.10.0",
			version: "1.10.0",
		},
		{
			desc:    "v1.10.0",
			version: "v1.10.0",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			fa := tools.NewOpenTofu(newOpenTofuRunner(), tC.version)
			if fa.SrcPath() != wantSrcPath {
				t.Errorf("Wrong opentofu src path: want = %s, got = %s", wantSrcPath, fa.SrcPath())
			}
		})
	}
}

func Test_OpenTofu_Rename(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "furyctl-test-")
	if err != nil {
		t.Fatalf("error creating temp dir: %v", err)
	}

	if _, err := os.Create(filepath.Join(tmpDir, "tofu")); err != nil {
		t.Fatalf("error creating temp file: %v", err)
	}

	fa := tools.NewOpenTofu(newOpenTofuRunner(), "1.10.0")

	if err := fa.Rename(tmpDir); err != nil {
		t.Fatalf("Error renaming opentofu binary: %v", err)
	}

	info, err := os.Stat(filepath.Join(tmpDir, "tofu"))
	if err != nil {
		t.Fatalf("Error stating opentofu binary: %v", err)
	}

	if info.IsDir() {
		t.Errorf("opentofu binary is a directory")
	}
}

func Test_OpenTofu_CheckBinVersion(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc        string
		wantVersion string
		wantErr     bool
		wantErrMsg  string
	}{
		{
			desc:        "correct version installed",
			wantVersion: "1.10.0",
		},
		{
			desc:        "wrong version installed",
			wantVersion: "1.8.0",
			wantErr:     true,
			wantErrMsg:  "installed = 1.10.0, expected = 1.8.0",
		},
	}
	for _, tC := range testCases {
		tC := tC

		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			fa := tools.NewOpenTofu(newOpenTofuRunner(), tC.wantVersion)

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

func newOpenTofuRunner() *opentofu.Runner {
	return opentofu.NewRunner(execx.NewFakeExecutor("TestHelperProcess"), opentofu.Paths{
		OpenTofu: "tofu",
	})
}
