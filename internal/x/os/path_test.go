// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package osx_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			dir, err := tC.setup()
			require.NoError(t, err, "error setting up test")

			err = osx.CleanupTempDir(dir)

			if tC.wantErr {
				require.Error(t, err, "expected error, got none")
			} else {
				require.NoError(t, err, "expected no errors")
			}

			_, err = os.Stat(dir)
			assert.ErrorIs(t, err, os.ErrNotExist, "expected directory to be removed")
		})
	}
}
