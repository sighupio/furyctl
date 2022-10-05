// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package osx_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	osx "github.com/sighupio/furyctl/internal/x/os"
)

func TestCleanupTempDir(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc    string
		setup   func() (string, error)
		wantErr bool
	}{
		{
			desc: "directory does not exist",
			setup: func() (string, error) {
				dir, err := os.MkdirTemp("", "furyctl")
				if err != nil {
					return "", err
				}

				return filepath.Join(dir, "non-existing-directory"), nil
			},
		},
		{
			desc: "directory exists",
			setup: func() (string, error) {
				return os.MkdirTemp("", "furyctl")
			},
		},
	}
	for _, tC := range testCases {
		tC := tC

		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			dir, err := tC.setup()
			if err != nil {
				t.Fatalf("error setting up test: %v", err)
			}

			err = osx.CleanupTempDir(dir)

			if tC.wantErr && err == nil {
				t.Fatalf("expected error, got none")
			}

			if !tC.wantErr && err != nil {
				t.Fatalf("expected no errors, got = %v", err)
			}

			if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("expected directory to be removed, got = %v", err)
			}
		})
	}
}
