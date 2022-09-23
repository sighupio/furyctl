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

func Test_Kustomize_SrcPath(t *testing.T) {
	wantSrcPath := fmt.Sprintf(
		"https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize/v3.10.0/kustomize_v3.10.0_%s_amd64.tar.gz",
		runtime.GOOS,
	)

	testCases := []struct {
		desc    string
		version string
	}{
		{
			desc:    "3.10.0",
			version: "3.10.0",
		},
		{
			desc:    "v3.10.0",
			version: "v3.10.0",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			fa := tools.NewKustomize(tC.version)
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

	fa := tools.NewKustomize("3.10.0")

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
