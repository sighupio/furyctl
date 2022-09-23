// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package distribution_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/netx"
	"github.com/sighupio/furyctl/internal/semver"
)

func Test_Downloader_Download(t *testing.T) {
	testCases := []struct {
		desc          string
		furyctlBinVer string
		wantApiVer    string
		wantKind      string
		wantDistroVer string
	}{
		{
			desc:          "unknown furyctl version",
			furyctlBinVer: "unknown",
			wantApiVer:    "kfd.sighup.io/v1alpha2",
			wantKind:      "EKSCluster",
			wantDistroVer: "v1.23.3",
		},
		{
			desc:          "compatible furyctl version",
			furyctlBinVer: "1.23.0",
			wantApiVer:    "kfd.sighup.io/v1alpha2",
			wantKind:      "EKSCluster",
			wantDistroVer: "v1.23.3",
		},
		{
			desc:          "older furyctl version",
			furyctlBinVer: "1.20.0",
			wantApiVer:    "kfd.sighup.io/v1alpha2",
			wantKind:      "EKSCluster",
			wantDistroVer: "v1.23.3",
		},
	}
	for _, tC := range testCases {
		tC := tC

		t.Run(tC.desc, func(t *testing.T) {
			distroLocation, err := filepath.Abs("../../test/data/distro/" + tC.wantDistroVer)
			if err != nil {
				t.Fatal(err)
			}

			d := distribution.NewDownloader(netx.NewGoGetterClient(), true)

			res, err := d.Download(tC.furyctlBinVer, distroLocation, "../../test/data/furyctl.yaml")
			if err != nil {
				t.Fatal(err)
			}

			if res.RepoPath == "" {
				t.Errorf("expected RepoPath, got empty string")
			}

			if res.MinimalConf.ApiVersion != tC.wantApiVer {
				t.Errorf("ApiVersion: want = %s, got = %s", tC.wantApiVer, res.MinimalConf.ApiVersion)
			}
			if res.MinimalConf.Kind.String() != tC.wantKind {
				t.Errorf("Kind: want = %s, got = %s", tC.wantKind, res.MinimalConf.Kind)
			}
			if res.MinimalConf.Spec.DistributionVersion != semver.Version(tC.wantDistroVer) {
				t.Errorf(
					"DistributionVersion: want = %s, got = %s",
					tC.wantDistroVer,
					res.MinimalConf.Spec.DistributionVersion,
				)
			}

			if res.DistroManifest.Version != semver.Version(tC.wantDistroVer) {
				t.Errorf("ApiVersion: want = %s, got = %s", tC.wantDistroVer, res.DistroManifest.Version)
			}
		})
	}
}

func TestGetSchemaPath(t *testing.T) {
	tests := []struct {
		name     string
		basePath string
		conf     distribution.FuryctlConfig
		want     string
		wantErr  error
	}{
		{
			name:     "test with base path",
			basePath: "testpath",
			conf: distribution.FuryctlConfig{
				ApiVersion: "kfd.sighup.io/v1alpha2",
				Kind:       "EKSCluster",
				Spec: struct {
					DistributionVersion semver.Version `yaml:"distributionVersion"`
				}{},
			},
			want: fmt.Sprintf("%s", filepath.Join(
				"testpath",
				"schemas",
				"ekscluster-kfd-v1alpha2.json",
			)),
			wantErr: nil,
		},
		{
			name:     "test without base path",
			basePath: "",
			conf: distribution.FuryctlConfig{
				ApiVersion: "kfd.sighup.io/v1alpha2",
				Kind:       "EKSCluster",
				Spec: struct {
					DistributionVersion semver.Version `yaml:"distributionVersion"`
				}{},
			},
			want:    fmt.Sprintf("%s", filepath.Join("schemas", "ekscluster-kfd-v1alpha2.json")),
			wantErr: nil,
		},
		{
			name:     "test with invalid apiVersion",
			basePath: "",
			conf: distribution.FuryctlConfig{
				ApiVersion: "",
				Kind:       "EKSCluster",
				Spec: struct {
					DistributionVersion semver.Version `yaml:"distributionVersion"`
				}{},
			},
			want:    "",
			wantErr: fmt.Errorf("invalid apiVersion: "),
		},
		{
			name:     "test with invalid kind",
			basePath: "",
			conf: distribution.FuryctlConfig{
				ApiVersion: "kfd.sighup.io/v1alpha2",
				Kind:       "",
				Spec: struct {
					DistributionVersion semver.Version `yaml:"distributionVersion"`
				}{},
			},
			want:    "",
			wantErr: fmt.Errorf("kind is empty"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := distribution.GetSchemaPath(tt.basePath, tt.conf)
			if err != nil {
				if err.Error() != tt.wantErr.Error() {
					t.Errorf("distribution.GetSchemaPath() error = %v, wantErr %v", err, tt.wantErr)
				}

				return
			}

			if got != tt.want {
				t.Errorf("distribution.GetSchemaPath() = %v, want %v", got, tt.want)
			}
		})
	}
}
