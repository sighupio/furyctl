// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package semver_test

import (
	"testing"

	"github.com/sighupio/furyctl/internal/semver"
)

func TestGetVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		version string
		wantErr bool
	}{
		{
			name:    "valid version",
			version: "v1.2.3",
			wantErr: false,
		},
		{
			name:    "valid next version",
			version: "v1.2.3-next",
			wantErr: false,
		},
		{
			name:    "valid alpha version",
			version: "v1.2.3-alpha",
			wantErr: false,
		},
		{
			name:    "valid beta version",
			version: "v1.2.3-beta.2",
			wantErr: false,
		},
		{
			name:    "valid rc version",
			version: "v1.2.3-rc.3",
			wantErr: false,
		},
		{
			name:    "valid v-less version",
			version: "1.2.3",
			wantErr: false,
		},
		{
			name:    "valid numeric version",
			version: "11",
			wantErr: false,
		},
		{
			name:    "invalid string version",
			version: "asd",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := semver.GetVersion(tt.version)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewVersion() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
		})
	}
}

func TestGetConstraint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		constraint string
		wantErr    bool
	}{
		{
			name:       "valid constraint",
			constraint: "v1.2.3",
			wantErr:    false,
		},
		{
			name:       "valid v-less constraint",
			constraint: "1.2.3",
			wantErr:    false,
		},
		{
			name:       "valid constraint with greater than",
			constraint: ">1.2.3",
			wantErr:    false,
		},
		{
			name:       "valid constraint with greater or equal than",
			constraint: ">=1.2.3",
			wantErr:    false,
		},
		{
			name:       "valid constraint with pessimistic",
			constraint: "~>1.2.3",
			wantErr:    false,
		},
		{
			name:       "valid constraint with wildcard",
			constraint: "*",
			wantErr:    false,
		},
		{
			name:       "invalid constraint",
			constraint: ">1.",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := semver.GetConstraint(tt.constraint)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewConstraint() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
		})
	}
}
