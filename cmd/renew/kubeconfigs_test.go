// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package renew

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSelectKubeconfigs(t *testing.T) {
	t.Parallel()

	available := []string{"admin", "alice", "bob"}

	tests := []struct {
		desc      string
		args      []string
		available []string
		want      []string
		wantErr   bool
	}{
		{desc: "no args selects all", args: nil, available: available, want: available},
		{desc: "empty arg selects all", args: []string{""}, available: available, want: available},
		{desc: "one name", args: []string{"alice"}, available: available, want: []string{"alice"}},
		{
			desc:      "space separated names",
			args:      []string{"admin", "bob"},
			available: available,
			want:      []string{"admin", "bob"},
		},
		{
			desc:      "comma separated names",
			args:      []string{"admin,bob"},
			available: available,
			want:      []string{"admin", "bob"},
		},
		{
			desc:      "comma and space mixed",
			args:      []string{"admin,", "bob"},
			available: available,
			want:      []string{"admin", "bob"},
		},
		{desc: "unknown name fails", args: []string{"alice", "carol"}, available: available, wantErr: true},
		{desc: "unsupported kind selects nothing", args: []string{"admin"}, available: nil, want: nil},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()

			got, err := selectKubeconfigs(tc.args, tc.available)
			if tc.wantErr {
				require.Error(t, err)
				assert.True(t, errors.Is(err, ErrUnknownKubeconfig))

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
