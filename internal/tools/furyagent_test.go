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

func Test_FuryAgent_SrcPath(t *testing.T) {
	wantSrcPath := fmt.Sprintf(
		"https://github.com/sighupio/furyagent/releases/download/v0.3.0/furyagent-%s-amd64",
		runtime.GOOS,
	)

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
			fa := tools.NewFuryAgent(tC.version)
			if fa.SrcPath() != wantSrcPath {
				t.Errorf("Wrong furyagent src path: want = %s, got = %s", wantSrcPath, fa.SrcPath())
			}
		})
	}
}

func Test_FuryAgent_Rename(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "furyctl-test-")
	if err != nil {
		t.Fatalf("error creating temp dir: %v", err)
	}

	if _, err := os.Create(filepath.Join(tmpDir, fmt.Sprintf("furyagent-%s-amd64", runtime.GOOS))); err != nil {
		t.Fatalf("error creating temp file: %v", err)
	}

	fa := tools.NewFuryAgent("0.3.0")

	if err := fa.Rename(tmpDir); err != nil {
		t.Fatalf("Error renaming furyagent binary: %v", err)
	}

	info, err := os.Stat(filepath.Join(tmpDir, "furyagent"))
	if err != nil {
		t.Fatalf("Error stating furyagent binary: %v", err)
	}

	if info.IsDir() {
		t.Errorf("furyagent binary is a directory")
	}
}
