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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/dependencies/tools"
	"github.com/sighupio/furyctl/internal/tool/furyagent"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

func Test_FuryAgent_SupportsDownload(t *testing.T) {
	a := tools.NewFuryagent(newFuryagentRunner(), "0.3.0")

	if a.SupportsDownload() != true {
		t.Errorf("Furyagent download must be supported")
	}
}

func Test_FuryAgent_SrcPath(t *testing.T) {
	wantSrcPath := fmt.Sprintf(
		"https://github.com/sighupio/furyagent/releases/download/v0.3.0/furyagent-%s-%s",
		runtime.GOOS,
		runtime.GOARCH,
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
			fa := tools.NewFuryagent(newFuryagentRunner(), tC.version)
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

	if _, err := os.Create(filepath.Join(tmpDir, fmt.Sprintf(
		"furyagent-%s-%s",
		runtime.GOOS,
		runtime.GOARCH,
	))); err != nil {
		t.Fatalf("error creating temp file: %v", err)
	}

	fa := tools.NewFuryagent(newFuryagentRunner(), "0.3.0")

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

func Test_Furyagent_CheckBinVersion(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc        string
		wantVersion string
		wantErr     bool
		wantErrMsg  string
	}{
		{
			desc:        "correct version installed",
			wantVersion: "0.3.0",
		},
		{
			desc:        "wrong version installed",
			wantVersion: "0.4.0",
			wantErr:     true,
			wantErrMsg:  "installed = 0.3.0, expected = 0.4.0",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			fa := tools.NewFuryagent(newFuryagentRunner(), tC.wantVersion)

			err := fa.CheckBinVersion()

			if tC.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tC.wantErrMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func newFuryagentRunner() *furyagent.Runner {
	return furyagent.NewRunner(execx.NewFakeExecutor("TestHelperProcess"), furyagent.Paths{
		Furyagent: "furyagent",
	})
}
