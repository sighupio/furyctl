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

func Test_Kubectl_SrcPath(t *testing.T) {
	wantSrcPath := fmt.Sprintf("https://dl.k8s.io/release/v1.23.10/bin/%s/amd64/kubectl", runtime.GOOS)

	testCases := []struct {
		desc    string
		version string
	}{
		{
			desc:    "1.23.10",
			version: "1.23.10",
		},
		{
			desc:    "v1.23.10",
			version: "v1.23.10",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			fa := tools.NewKubectl(tC.version)
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

	fa := tools.NewKubectl("1.23.10")

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
