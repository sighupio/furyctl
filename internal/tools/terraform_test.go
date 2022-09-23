// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tools_test

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/sighupio/furyctl/internal/tools"
)

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
			fa := tools.NewTerraform(tC.version)
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

	fa := tools.NewTerraform("1.2.9")

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
