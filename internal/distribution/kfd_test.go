// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package distribution_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/distribution"
)

func TestKFDTemplateData(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc    string
		kfd     *string
		want    map[any]any
		wantErr bool
	}{
		{
			desc: "returns version and every module, including ones not modeled in config.KFD",
			kfd: new(`version: v1.36.0
modules:
  monitoring: v4.3.0
  utilities: v0.1.1
  futuremodule: v0.0.1
`),
			want: map[any]any{
				"version": "v1.36.0",
				"modules": map[any]any{
					"monitoring":   "v4.3.0",
					"utilities":    "v0.1.1",
					"futuremodule": "v0.0.1",
				},
			},
		},
		{
			desc:    "fails when kfd.yaml is missing",
			wantErr: true,
		},
		{
			desc:    "fails when kfd.yaml is not valid yaml",
			kfd:     new("version: [v1.36.0"),
			wantErr: true,
		},
	}

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			distroPath := t.TempDir()

			if tC.kfd != nil {
				require.NoError(t, os.WriteFile(filepath.Join(distroPath, "kfd.yaml"), []byte(*tC.kfd), 0o600))
			}

			got, err := distribution.KFDTemplateData(distroPath)

			if tC.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			require.Equal(t, tC.want, got)
		})
	}
}
