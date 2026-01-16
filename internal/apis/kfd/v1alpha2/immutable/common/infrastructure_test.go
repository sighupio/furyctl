// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package common_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/immutable/common"
	"github.com/sighupio/furyctl/internal/cluster"
	iox "github.com/sighupio/furyctl/internal/x/io"
)

func Test_Infrastructure_CreateFolderStructure(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc       string
		setupFunc  func(string) error
		wantErr    bool
		wantErrMsg string
	}{
		{
			desc: "creates folder structure successfully",
		},
		{
			desc: "handles existing folders gracefully",
			setupFunc: func(basePath string) error {
				// Pre-create one of the folders.
				return os.MkdirAll(filepath.Join(basePath, "templates", "butane", "install"), iox.FullPermAccess)
			},
		},
	}

	for _, tC := range testCases {
		tC := tC
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			// Create temporary directory for test.
			tmpDir, err := os.MkdirTemp("", "furyctl-test-*")
			if err != nil {
				t.Fatalf("failed to create temp dir: %v", err)
			}

			defer os.RemoveAll(tmpDir)

			if tC.setupFunc != nil {
				if err := tC.setupFunc(tmpDir); err != nil {
					t.Fatalf("setup failed: %v", err)
				}
			}

			infra := &common.Infrastructure{
				OperationPhase: &cluster.OperationPhase{
					Path: tmpDir,
				},
			}

			err = infra.CreateFolderStructure()

			if tC.wantErr && err == nil {
				t.Errorf("expected error, got nil")
			}

			if !tC.wantErr && err != nil {
				t.Errorf("expected nil, got error: %v", err)
			}

			if tC.wantErr && err != nil && err.Error() != tC.wantErrMsg {
				t.Errorf("expected error message '%s', got '%s'", tC.wantErrMsg, err.Error())
			}

			// Verify folders were created.
			if !tC.wantErr {
				expectedFolders := []string{
					filepath.Join(tmpDir, "templates", "butane", "install"),
					filepath.Join(tmpDir, "templates", "butane", "bootstrap"),
					filepath.Join(tmpDir, "server", "ignition"),
					filepath.Join(tmpDir, "server", "assets", "flatcar"),
					filepath.Join(tmpDir, "server", "assets", "extensions"),
				}

				for _, folder := range expectedFolders {
					if _, err := os.Stat(folder); os.IsNotExist(err) {
						t.Errorf("expected folder %s to exist, but it doesn't", folder)
					}
				}
			}
		})
	}
}
