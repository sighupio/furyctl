// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package semver_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
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

func TestGt(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc string
		a    string
		b    string
		want bool
	}{
		{
			desc: "a equals b",
			a:    "0.1.0",
			b:    "0.1.0",
			want: false,
		},
		{
			desc: "a lesser patch than b",
			a:    "0.1.0",
			b:    "0.1.1",
			want: false,
		},
		{
			desc: "a greater patch than b",
			a:    "0.1.1",
			b:    "0.1.0",
			want: true,
		},
		{
			desc: "a lesser minor than b",
			a:    "0.1.0",
			b:    "0.2.0",
			want: false,
		},
		{
			desc: "a greater minor than b",
			a:    "0.2.0",
			b:    "0.1.0",
			want: true,
		},
		{
			desc: "a lesser major than b",
			a:    "0.1.0",
			b:    "1.1.0",
			want: false,
		},
		{
			desc: "a greater major than b",
			a:    "1.1.0",
			b:    "0.1.0",
			want: true,
		},
		{
			desc: "a lesser major, greater minor and patch than b",
			a:    "0.2.2",
			b:    "1.0.0",
			want: false,
		},
		{
			desc: "a lesser minor, greater patch than b",
			a:    "0.2.2",
			b:    "0.3.0",
			want: false,
		},
	}
	for _, tC := range testCases {
		tC := tC

		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			if got := semver.Gt(tC.a, tC.b); got != tC.want {
				t.Errorf("Gt() = %v, want %v", got, tC.want)
			}
		})
	}
}

func TestParts(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc    string
		version string
		want    []any
	}{
		{desc: "0.0.4", want: []any{0, 0, 4, ""}},
		{desc: "1.2.3", want: []any{1, 2, 3, ""}},
		{desc: "10.20.30", want: []any{10, 20, 30, ""}},
		{desc: "1.1.2-prerelease+meta", want: []any{1, 1, 2, "prerelease+meta"}},
		{desc: "1.1.2+meta", want: []any{1, 1, 2, "meta"}},
		{desc: "1.1.2+meta-valid", want: []any{1, 1, 2, "meta-valid"}},
		{desc: "1.0.0-alpha", want: []any{1, 0, 0, "alpha"}},
		{desc: "1.0.0-beta", want: []any{1, 0, 0, "beta"}},
		{desc: "1.0.0-alpha.beta", want: []any{1, 0, 0, "alpha.beta"}},
		{desc: "1.0.0-alpha.beta.1", want: []any{1, 0, 0, "alpha.beta.1"}},
		{desc: "1.0.0-alpha.1", want: []any{1, 0, 0, "alpha.1"}},
		{desc: "1.0.0-alpha0.valid", want: []any{1, 0, 0, "alpha0.valid"}},
		{desc: "1.0.0-alpha.0valid", want: []any{1, 0, 0, "alpha.0valid"}},
		{desc: "1.0.0-alpha-a.b-c-somethinglong+build.1-aef.1-its-okay", want: []any{1, 0, 0, "alpha-a.b-c-somethinglong+build.1-aef.1-its-okay"}},
		{desc: "1.0.0-rc.1+build.1", want: []any{1, 0, 0, "rc.1+build.1"}},
		{desc: "2.0.0-rc.1+build.123", want: []any{2, 0, 0, "rc.1+build.123"}},
		{desc: "1.2.3-beta", want: []any{1, 2, 3, "beta"}},
		{desc: "10.2.3-DEV-SNAPSHOT", want: []any{10, 2, 3, "DEV-SNAPSHOT"}},
		{desc: "1.2.3-SNAPSHOT-123", want: []any{1, 2, 3, "SNAPSHOT-123"}},
		{desc: "1.0.0", want: []any{1, 0, 0, ""}},
		{desc: "2.0.0", want: []any{2, 0, 0, ""}},
		{desc: "1.1.7", want: []any{1, 1, 7, ""}},
		{desc: "2.0.0+build.1848", want: []any{2, 0, 0, "build.1848"}},
		{desc: "2.0.1-alpha.1227", want: []any{2, 0, 1, "alpha.1227"}},
		{desc: "1.0.0-alpha+beta", want: []any{1, 0, 0, "alpha+beta"}},
		{desc: "1.2.3----RC-SNAPSHOT.12.9.1--.12+788", want: []any{1, 2, 3, "---RC-SNAPSHOT.12.9.1--.12+788"}},
		{desc: "1.2.3----R-S.12.9.1--.12+meta", want: []any{1, 2, 3, "---R-S.12.9.1--.12+meta"}},
		{desc: "1.2.3----RC-SNAPSHOT.12.9.1--.12", want: []any{1, 2, 3, "---RC-SNAPSHOT.12.9.1--.12"}},
		{desc: "1.0.0+0.build.1-rc.10000aaa-kk-0.1", want: []any{1, 0, 0, "0.build.1-rc.10000aaa-kk-0.1"}},
		{desc: "999999999999999999.999999999999999999.99999999999999999", want: []any{999999999999999999, 999999999999999999, 99999999999999999, ""}},
		{desc: "1.0.0-0A.is.legal", want: []any{1, 0, 0, "0A.is.legal"}},

		{desc: "1", want: []any{0, 0, 0, ""}},
		{desc: "1.2", want: []any{0, 0, 0, ""}},
		{desc: "1.2.3-0123", want: []any{0, 0, 0, ""}},
		{desc: "1.2.3-0123.0123", want: []any{0, 0, 0, ""}},
		{desc: "1.1.2+.123", want: []any{0, 0, 0, ""}},
		{desc: "+invalid", want: []any{0, 0, 0, ""}},
		{desc: "-invalid", want: []any{0, 0, 0, ""}},
		{desc: "-invalid+invalid", want: []any{0, 0, 0, ""}},
		{desc: "-invalid.01", want: []any{0, 0, 0, ""}},
		{desc: "alpha", want: []any{0, 0, 0, ""}},
		{desc: "alpha.beta", want: []any{0, 0, 0, ""}},
		{desc: "alpha.beta.1", want: []any{0, 0, 0, ""}},
		{desc: "alpha.1", want: []any{0, 0, 0, ""}},
		{desc: "alpha+beta", want: []any{0, 0, 0, ""}},
		{desc: "alpha_beta", want: []any{0, 0, 0, ""}},
		{desc: "alpha.", want: []any{0, 0, 0, ""}},
		{desc: "alpha..", want: []any{0, 0, 0, ""}},
		{desc: "beta", want: []any{0, 0, 0, ""}},
		{desc: "1.0.0-alpha_beta", want: []any{0, 0, 0, ""}},
		{desc: "-alpha.", want: []any{0, 0, 0, ""}},
		{desc: "1.0.0-alpha..", want: []any{0, 0, 0, ""}},
		{desc: "1.0.0-alpha..1", want: []any{0, 0, 0, ""}},
		{desc: "1.0.0-alpha...1", want: []any{0, 0, 0, ""}},
		{desc: "1.0.0-alpha....1", want: []any{0, 0, 0, ""}},
		{desc: "1.0.0-alpha.....1", want: []any{0, 0, 0, ""}},
		{desc: "1.0.0-alpha......1", want: []any{0, 0, 0, ""}},
		{desc: "1.0.0-alpha.......1", want: []any{0, 0, 0, ""}},
		{desc: "01.1.1", want: []any{0, 0, 0, ""}},
		{desc: "1.01.1", want: []any{0, 0, 0, ""}},
		{desc: "1.1.01", want: []any{0, 0, 0, ""}},
		{desc: "1.2", want: []any{0, 0, 0, ""}},
		{desc: "1.2.3.DEV", want: []any{0, 0, 0, ""}},
		{desc: "1.2-SNAPSHOT", want: []any{0, 0, 0, ""}},
		{desc: "1.2.31.2.3----RC-SNAPSHOT.12.09.1--..12+788", want: []any{0, 0, 0, ""}},
		{desc: "1.2-RC-SNAPSHOT", want: []any{0, 0, 0, ""}},
		{desc: "-1.0.3-gamma+b7718", want: []any{0, 0, 0, ""}},
		{desc: "+justmeta", want: []any{0, 0, 0, ""}},
		{desc: "9.8.7+meta+meta", want: []any{0, 0, 0, ""}},
		{desc: "9.8.7-whatever+meta+meta", want: []any{0, 0, 0, ""}},
		{desc: "99999999999999999999999.999999999999999999.99999999999999999----RC-SNAPSHOT.12.09.1--------------------------------..12", want: []any{0, 0, 0, ""}},
	}
	for _, tC := range testCases {
		tC := tC

		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			major, minor, patch, rest := semver.Parts(tC.desc)

			parts := []any{major, minor, patch, rest}

			if !cmp.Equal(parts, tC.want) {
				t.Errorf("parts mismatch (-want +got):\n%s", cmp.Diff(tC.want, parts))
			}
		})
	}
}
