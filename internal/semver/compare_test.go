// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package semver_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/sighupio/furyctl/internal/semver"
)

func Test_NewVersion(t *testing.T) {
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
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

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
		want    semver.VersionParts
	}{
		{desc: "0.0.4", want: semver.VersionParts{Comparator: semver.Equal{}, Patch: 4}},
		{desc: "1.2.3", want: semver.VersionParts{Comparator: semver.Equal{}, Major: 1, Minor: 2, Patch: 3}},
		{desc: "10.20.30", want: semver.VersionParts{Comparator: semver.Equal{}, Major: 10, Minor: 20, Patch: 30}},
		{desc: "1.1.2-prerelease+meta", want: semver.VersionParts{Comparator: semver.Equal{}, Major: 1, Minor: 1, Patch: 2, Suffix: "prerelease+meta"}},
		{desc: "1.1.2+meta", want: semver.VersionParts{Comparator: semver.Equal{}, Major: 1, Minor: 1, Patch: 2, Suffix: "meta"}},
		{desc: "1.1.2+meta-valid", want: semver.VersionParts{Comparator: semver.Equal{}, Major: 1, Minor: 1, Patch: 2, Suffix: "meta-valid"}},
		{desc: "1.0.0-alpha", want: semver.VersionParts{Comparator: semver.Equal{}, Major: 1, Suffix: "alpha"}},
		{desc: "1.0.0-beta", want: semver.VersionParts{Comparator: semver.Equal{}, Major: 1, Suffix: "beta"}},
		{desc: "1.0.0-alpha.beta", want: semver.VersionParts{Comparator: semver.Equal{}, Major: 1, Suffix: "alpha.beta"}},
		{desc: "1.0.0-alpha.beta.1", want: semver.VersionParts{Comparator: semver.Equal{}, Major: 1, Suffix: "alpha.beta.1"}},
		{desc: "1.0.0-alpha.1", want: semver.VersionParts{Comparator: semver.Equal{}, Major: 1, Suffix: "alpha.1"}},
		{desc: "1.0.0-alpha0.valid", want: semver.VersionParts{Comparator: semver.Equal{}, Major: 1, Suffix: "alpha0.valid"}},
		{desc: "1.0.0-alpha.0valid", want: semver.VersionParts{Comparator: semver.Equal{}, Major: 1, Suffix: "alpha.0valid"}},
		{desc: "1.0.0-alpha-a.b-c-somethinglong+build.1-aef.1-its-okay", want: semver.VersionParts{Comparator: semver.Equal{}, Major: 1, Suffix: "alpha-a.b-c-somethinglong+build.1-aef.1-its-okay"}},
		{desc: "1.0.0-rc.1+build.1", want: semver.VersionParts{Comparator: semver.Equal{}, Major: 1, Suffix: "rc.1+build.1"}},
		{desc: "2.0.0-rc.1+build.123", want: semver.VersionParts{Comparator: semver.Equal{}, Major: 2, Suffix: "rc.1+build.123"}},
		{desc: "1.2.3-beta", want: semver.VersionParts{Comparator: semver.Equal{}, Major: 1, Minor: 2, Patch: 3, Suffix: "beta"}},
		{desc: "10.2.3-DEV-SNAPSHOT", want: semver.VersionParts{Comparator: semver.Equal{}, Major: 10, Minor: 2, Patch: 3, Suffix: "DEV-SNAPSHOT"}},
		{desc: "1.2.3-SNAPSHOT-123", want: semver.VersionParts{Comparator: semver.Equal{}, Major: 1, Minor: 2, Patch: 3, Suffix: "SNAPSHOT-123"}},
		{desc: "1.0.0", want: semver.VersionParts{Comparator: semver.Equal{}, Major: 1}},
		{desc: "2.0.0", want: semver.VersionParts{Comparator: semver.Equal{}, Major: 2}},
		{desc: "1.1.7", want: semver.VersionParts{Comparator: semver.Equal{}, Major: 1, Minor: 1, Patch: 7}},
		{desc: "2.0.0+build.1848", want: semver.VersionParts{Comparator: semver.Equal{}, Major: 2, Suffix: "build.1848"}},
		{desc: "<2.0.0+build.1848", want: semver.VersionParts{Comparator: semver.Less{}, Major: 2, Suffix: "build.1848"}},
		{desc: "<=2.0.0+build.1848", want: semver.VersionParts{Comparator: semver.LessOrEqual{}, Major: 2, Suffix: "build.1848"}},
		{desc: ">2.0.0+build.1848", want: semver.VersionParts{Comparator: semver.Greater{}, Major: 2, Suffix: "build.1848"}},
		{desc: ">=2.0.0+build.1848", want: semver.VersionParts{Comparator: semver.GreaterOrEqual{}, Major: 2, Suffix: "build.1848"}},
		{desc: "~2.0.0+build.1848", want: semver.VersionParts{Comparator: semver.CompatibleUp{}, Major: 2, Suffix: "build.1848"}},
		{desc: "^2.0.0+build.1848", want: semver.VersionParts{Comparator: semver.Compatible{}, Major: 2, Suffix: "build.1848"}},
		{desc: "2.0.1-alpha.1227", want: semver.VersionParts{Comparator: semver.Equal{}, Major: 2, Patch: 1, Suffix: "alpha.1227"}},
		{desc: "1.0.0-alpha+beta", want: semver.VersionParts{Comparator: semver.Equal{}, Major: 1, Suffix: "alpha+beta"}},
		{desc: "1.2.3----RC-SNAPSHOT.12.9.1--.12+788", want: semver.VersionParts{Comparator: semver.Equal{}, Major: 1, Minor: 2, Patch: 3, Suffix: "---RC-SNAPSHOT.12.9.1--.12+788"}},
		{desc: "1.2.3----R-S.12.9.1--.12+meta", want: semver.VersionParts{Comparator: semver.Equal{}, Major: 1, Minor: 2, Patch: 3, Suffix: "---R-S.12.9.1--.12+meta"}},
		{desc: "1.2.3----RC-SNAPSHOT.12.9.1--.12", want: semver.VersionParts{Comparator: semver.Equal{}, Major: 1, Minor: 2, Patch: 3, Suffix: "---RC-SNAPSHOT.12.9.1--.12"}},
		{desc: "1.0.0+0.build.1-rc.10000aaa-kk-0.1", want: semver.VersionParts{Comparator: semver.Equal{}, Major: 1, Suffix: "0.build.1-rc.10000aaa-kk-0.1"}},
		{desc: "999999999999999999.999999999999999999.99999999999999999", want: semver.VersionParts{Comparator: semver.Equal{}, Major: 999999999999999999, Minor: 999999999999999999, Patch: 99999999999999999}},
		{desc: "1.0.0-0A.is.legal", want: semver.VersionParts{Comparator: semver.Equal{}, Major: 1, Suffix: "0A.is.legal"}},
		{desc: "*", want: semver.VersionParts{Comparator: semver.Always{}}},

		{desc: "1", want: semver.VersionParts{}},
		{desc: "1.2", want: semver.VersionParts{}},
		{desc: "1.2.3-0123", want: semver.VersionParts{}},
		{desc: "1.2.3-0123.0123", want: semver.VersionParts{}},
		{desc: "1.1.2+.123", want: semver.VersionParts{}},
		{desc: "+invalid", want: semver.VersionParts{}},
		{desc: "-invalid", want: semver.VersionParts{}},
		{desc: "-invalid+invalid", want: semver.VersionParts{}},
		{desc: "-invalid.01", want: semver.VersionParts{}},
		{desc: "alpha", want: semver.VersionParts{}},
		{desc: "alpha.beta", want: semver.VersionParts{}},
		{desc: "alpha.beta.1", want: semver.VersionParts{}},
		{desc: "alpha.1", want: semver.VersionParts{}},
		{desc: "alpha+beta", want: semver.VersionParts{}},
		{desc: "alpha_beta", want: semver.VersionParts{}},
		{desc: "alpha.", want: semver.VersionParts{}},
		{desc: "alpha..", want: semver.VersionParts{}},
		{desc: "beta", want: semver.VersionParts{}},
		{desc: "1.0.0-alpha_beta", want: semver.VersionParts{}},
		{desc: "-alpha.", want: semver.VersionParts{}},
		{desc: "1.0.0-alpha..", want: semver.VersionParts{}},
		{desc: "1.0.0-alpha..1", want: semver.VersionParts{}},
		{desc: "1.0.0-alpha...1", want: semver.VersionParts{}},
		{desc: "1.0.0-alpha....1", want: semver.VersionParts{}},
		{desc: "1.0.0-alpha.....1", want: semver.VersionParts{}},
		{desc: "1.0.0-alpha......1", want: semver.VersionParts{}},
		{desc: "1.0.0-alpha.......1", want: semver.VersionParts{}},
		{desc: "01.1.1", want: semver.VersionParts{}},
		{desc: "1.01.1", want: semver.VersionParts{}},
		{desc: "1.1.01", want: semver.VersionParts{}},
		{desc: "1.2", want: semver.VersionParts{}},
		{desc: "1.2.3.DEV", want: semver.VersionParts{}},
		{desc: "1.2-SNAPSHOT", want: semver.VersionParts{}},
		{desc: "1.2.31.2.3----RC-SNAPSHOT.12.09.1--..12+788", want: semver.VersionParts{}},
		{desc: "1.2-RC-SNAPSHOT", want: semver.VersionParts{}},
		{desc: "-1.0.3-gamma+b7718", want: semver.VersionParts{}},
		{desc: "+justmeta", want: semver.VersionParts{}},
		{desc: "9.8.7+meta+meta", want: semver.VersionParts{}},
		{desc: "9.8.7-whatever+meta+meta", want: semver.VersionParts{}},
		{desc: "99999999999999999999999.999999999999999999.99999999999999999----RC-SNAPSHOT.12.09.1--------------------------------..12", want: semver.VersionParts{}},
	}
	for _, tC := range testCases {
		tC := tC

		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			parts := semver.Parts(tC.desc)

			if !cmp.Equal(parts, tC.want) {
				t.Errorf("parts mismatch (-want +got):\n%s", cmp.Diff(tC.want, parts))
			}
		})
	}
}

func TestSamePatchStr(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc string
		a    string
		b    string
		want bool
	}{
		{
			desc: "same patch, different build using dash",
			a:    "v1.2.3-1",
			b:    "v1.2.3-2",
			want: true,
		},
		{
			desc: "same patch, different build using plus",
			a:    "v1.2.3+b1",
			b:    "v1.2.3+b2",
			want: true,
		},
		{
			desc: "same patch",
			a:    "v1.2.3",
			b:    "v1.2.3",
			want: true,
		},
		{
			desc: "same minor",
			a:    "v1.2.3",
			b:    "v1.2.4",
			want: false,
		},
		{
			desc: "same major",
			a:    "v1.2.3",
			b:    "v1.3.4",
			want: false,
		},
	}
	for _, tC := range testCases {
		tC := tC

		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			got := semver.SamePatchStr(tC.a, tC.b)
			if got != tC.want {
				t.Errorf("want = %t, got = %t", tC.want, got)
			}
		})
	}
}

func TestSameMinorStr(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc string
		a    string
		b    string
		want bool
	}{
		{
			desc: "same patch, different build using dash",
			a:    "v1.2.3-1",
			b:    "v1.2.3-2",
			want: true,
		},
		{
			desc: "same patch, different build using plus",
			a:    "v1.2.3+b1",
			b:    "v1.2.3+b2",
			want: true,
		},
		{
			desc: "same patch",
			a:    "v1.2.3",
			b:    "v1.2.3",
			want: true,
		},
		{
			desc: "same minor",
			a:    "v1.2.3",
			b:    "v1.2.4",
			want: true,
		},
		{
			desc: "same major",
			a:    "v1.2.3",
			b:    "v1.3.4",
			want: false,
		},
	}
	for _, tC := range testCases {
		tC := tC

		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			got := semver.SameMinorStr(tC.a, tC.b)
			if got != tC.want {
				t.Errorf("want = %t, got = %t", tC.want, got)
			}
		})
	}
}

func TestVersionParts_CheckCompatibility(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc           string
		versionPattern semver.VersionParts
		version        semver.VersionParts
		want           bool
	}{
		{
			desc:           "check equal pattern true",
			versionPattern: semver.VersionParts{Comparator: semver.Equal{}, Major: 1, Minor: 2, Patch: 3},
			version:        semver.VersionParts{Major: 1, Minor: 2, Patch: 3},
			want:           true,
		},
		{
			desc:           "check equal pattern false",
			versionPattern: semver.VersionParts{Comparator: semver.Equal{}, Major: 1, Minor: 2, Patch: 3},
			version:        semver.VersionParts{Major: 1, Minor: 2, Patch: 4},
			want:           false,
		},
		{
			desc:           "check greater than pattern true",
			versionPattern: semver.VersionParts{Comparator: semver.Greater{}, Major: 1, Minor: 2, Patch: 3},
			version:        semver.VersionParts{Major: 1, Minor: 2, Patch: 4},
			want:           true,
		},
		{
			desc:           "check greater than pattern false",
			versionPattern: semver.VersionParts{Comparator: semver.Greater{}, Major: 1, Minor: 2, Patch: 3},
			version:        semver.VersionParts{Major: 1, Minor: 2, Patch: 2},
			want:           false,
		},
		{
			desc:           "check greater than or equal pattern true equal",
			versionPattern: semver.VersionParts{Comparator: semver.GreaterOrEqual{}, Major: 1, Minor: 2, Patch: 3},
			version:        semver.VersionParts{Major: 1, Minor: 2, Patch: 3},
			want:           true,
		},
		{
			desc:           "check greater than or equal pattern true greater",
			versionPattern: semver.VersionParts{Comparator: semver.GreaterOrEqual{}, Major: 1, Minor: 2, Patch: 3},
			version:        semver.VersionParts{Major: 1, Minor: 2, Patch: 4},
			want:           true,
		},
		{
			desc:           "check greater than or equal pattern false",
			versionPattern: semver.VersionParts{Comparator: semver.GreaterOrEqual{}, Major: 1, Minor: 2, Patch: 3},
			version:        semver.VersionParts{Major: 1, Minor: 2, Patch: 2},
			want:           false,
		},
		{
			desc:           "check less than pattern true",
			versionPattern: semver.VersionParts{Comparator: semver.Less{}, Major: 1, Minor: 2, Patch: 3},
			version:        semver.VersionParts{Major: 1, Minor: 2, Patch: 2},
			want:           true,
		},
		{
			desc:           "check less than pattern false",
			versionPattern: semver.VersionParts{Comparator: semver.Less{}, Major: 1, Minor: 2, Patch: 3},
			version:        semver.VersionParts{Major: 1, Minor: 2, Patch: 4},
			want:           false,
		},
		{
			desc:           "check less than or equal pattern true equal",
			versionPattern: semver.VersionParts{Comparator: semver.LessOrEqual{}, Major: 1, Minor: 2, Patch: 3},
			version:        semver.VersionParts{Major: 1, Minor: 2, Patch: 3},
			want:           true,
		},
		{
			desc:           "check less than or equal pattern true less",
			versionPattern: semver.VersionParts{Comparator: semver.LessOrEqual{}, Major: 1, Minor: 2, Patch: 3},
			version:        semver.VersionParts{Major: 1, Minor: 2, Patch: 2},
			want:           true,
		},
		{
			desc:           "check less than or equal pattern false",
			versionPattern: semver.VersionParts{Comparator: semver.LessOrEqual{}, Major: 1, Minor: 2, Patch: 3},
			version:        semver.VersionParts{Major: 1, Minor: 2, Patch: 4},
			want:           false,
		},
		{
			desc:           "check ^ pattern true build suffix",
			versionPattern: semver.VersionParts{Comparator: semver.Compatible{}, Major: 1, Minor: 2, Patch: 3},
			version:        semver.VersionParts{Major: 1, Minor: 2, Patch: 3, Suffix: "rc1"},
			want:           true,
		},
		{
			desc:           "check ^ pattern true patch",
			versionPattern: semver.VersionParts{Comparator: semver.Compatible{}, Major: 1, Minor: 2, Patch: 3},
			version:        semver.VersionParts{Major: 1, Minor: 2, Patch: 4},
			want:           true,
		},
		{
			desc:           "check ^ pattern true minor",
			versionPattern: semver.VersionParts{Comparator: semver.Compatible{}, Major: 1, Minor: 2, Patch: 3},
			version:        semver.VersionParts{Major: 1, Minor: 6, Patch: 4},
			want:           true,
		},
		{
			desc:           "check ^ pattern false",
			versionPattern: semver.VersionParts{Comparator: semver.Compatible{}, Major: 1, Minor: 2, Patch: 3},
			version:        semver.VersionParts{Major: 2, Minor: 2, Patch: 4},
			want:           false,
		},
		{
			desc:           "check ~ pattern true build suffix",
			versionPattern: semver.VersionParts{Comparator: semver.CompatibleUp{}, Major: 1, Minor: 2, Patch: 3},
			version:        semver.VersionParts{Major: 1, Minor: 2, Patch: 3, Suffix: "rc1"},
			want:           true,
		},
		{
			desc:           "check ~ pattern true patch",
			versionPattern: semver.VersionParts{Comparator: semver.CompatibleUp{}, Major: 1, Minor: 2, Patch: 3},
			version:        semver.VersionParts{Major: 1, Minor: 2, Patch: 4},
			want:           true,
		},
		{
			desc:           "check ~ pattern false",
			versionPattern: semver.VersionParts{Comparator: semver.CompatibleUp{}, Major: 1, Minor: 2, Patch: 3},
			version:        semver.VersionParts{Major: 1, Minor: 6, Patch: 4},
			want:           false,
		},
		{
			desc:           "check * pattern true",
			versionPattern: semver.VersionParts{Comparator: semver.Always{}, Major: 1, Minor: 2, Patch: 3},
			version:        semver.VersionParts{Major: 1, Minor: 2, Patch: 4},
			want:           true,
		},
	}

	for _, tC := range testCases {
		tC := tC

		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			got := tC.versionPattern.CheckCompatibility(tC.version)
			if got != tC.want {
				t.Errorf("want = %t, got = %t", tC.want, got)
			}
		})
	}
}
