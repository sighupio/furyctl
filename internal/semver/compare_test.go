// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package semver_test

import (
	"testing"

	"github.com/sighupio/furyctl/internal/semver"
)

func Test_NewVersion(t *testing.T) {
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
			name:    "invalid v-less version",
			version: "1.2.3",
			wantErr: true,
		},
		{
			name:    "invalid numeric version",
			version: "11",
			wantErr: true,
		},
		{
			name:    "invalid string version",
			version: "asd",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := semver.NewVersion(tt.version)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewVersion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}
