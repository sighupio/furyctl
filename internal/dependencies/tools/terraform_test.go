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
	"github.com/sighupio/furyctl/internal/tool/terraform"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

func Test_Terraform_SupportsDownload(t *testing.T) {
	a := tools.NewTerraform(newTerraformRunner(), "3.10.0")

	if a.SupportsDownload() != true {
		t.Errorf("terraform download must be supported")
	}
}

func Test_Terraform_SrcPath(t *testing.T) {
	wantSrcPath := fmt.Sprintf(
		"https://releases.hashicorp.com/terraform/1.2.9/terraform_1.2.9_%s_amd64.zip",
		runtime.GOOS,
	)

	testCases := []struct {
		desc    string
		version string
	}{
		{
			desc:    "1.2.9",
			version: "1.2.9",
		},
		{
			desc:    "v1.2.9",
			version: "v1.2.9",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			fa := tools.NewTerraform(newTerraformRunner(), tC.version)
			if fa.SrcPath() != wantSrcPath {
				t.Errorf("Wrong terraform src path: want = %s, got = %s", wantSrcPath, fa.SrcPath())
			}
		})
	}
}

func Test_Terraform_Rename(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "furyctl-test-")
	if err != nil {
		t.Fatalf("error creating temp dir: %v", err)
	}

	if _, err := os.Create(filepath.Join(tmpDir, "terraform")); err != nil {
		t.Fatalf("error creating temp file: %v", err)
	}

	fa := tools.NewTerraform(newTerraformRunner(), "1.2.9")

	if err := fa.Rename(tmpDir); err != nil {
		t.Fatalf("Error renaming terraform binary: %v", err)
	}

	info, err := os.Stat(filepath.Join(tmpDir, "terraform"))
	if err != nil {
		t.Fatalf("Error stating terraform binary: %v", err)
	}

	if info.IsDir() {
		t.Errorf("terraform binary is a directory")
	}
}

func Test_Terraform_CheckBinVersion(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc        string
		wantVersion string
		wantErr     bool
		wantErrMsg  string
	}{
		{
			desc:        "correct version installed",
			wantVersion: "0.15.4",
		},
		{
			desc:        "wrong version installed",
			wantVersion: "1.3.0",
			wantErr:     true,
			wantErrMsg:  "installed = 0.15.4, expected = 1.3.0",
		},
	}
	for _, tC := range testCases {
		tC := tC

		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			fa := tools.NewTerraform(newTerraformRunner(), tC.wantVersion)

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

func newTerraformRunner() *terraform.Runner {
	return terraform.NewRunner(execx.NewFakeExecutor(), terraform.Paths{
		Terraform: "terraform",
	})
}
