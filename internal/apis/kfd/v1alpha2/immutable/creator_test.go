// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package immutable_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/immutable"
)

func Test_ClusterCreator_GetPhasePath(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc     string
		phase    string
		wantPath string
		wantErr  bool
	}{
		{
			desc:     "empty phase (all phases) returns the all-phase schema path",
			phase:    "",
			wantPath: "",
		},
		{
			desc:     "infrastructure phase",
			phase:    "infrastructure",
			wantPath: ".spec.infrastructure",
		},
		{
			desc:     "kubernetes phase",
			phase:    "kubernetes",
			wantPath: ".spec.kubernetes",
		},
		{
			desc:     "distribution phase",
			phase:    "distribution",
			wantPath: ".spec.distribution",
		},
		{
			desc:     "plugins phase",
			phase:    "plugins",
			wantPath: ".spec.plugins",
		},
		{
			desc:    "unsupported phase",
			phase:   "bogus",
			wantErr: true,
		},
	}

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			creator := &immutable.ClusterCreator{}

			path, err := creator.GetPhasePath(tC.phase)

			if tC.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tC.wantPath, path)
		})
	}
}
