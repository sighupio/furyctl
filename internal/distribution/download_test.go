// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build integration

package distribution_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/sighupio/furyctl/internal/distribution"
	netx "github.com/sighupio/furyctl/internal/x/net"
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
			distroPath := fmt.Sprintf("../../test/data/integration/%s/distro", tC.wantDistroVer)
			absDistroPath, err := filepath.Abs(distroPath)
			if err != nil {
				t.Fatal(err)
			}

			d := distribution.NewDownloader(netx.NewGoGetterClient())

			res, err := d.Download(
				tC.furyctlBinVer,
				absDistroPath,
				fmt.Sprintf("../../test/data/integration/%s/furyctl.yaml", tC.wantDistroVer),
			)
			if err != nil {
				t.Fatal(err)
			}

			if res.RepoPath == "" {
				t.Errorf("expected RepoPath, got empty string")
			}

			if res.MinimalConf.APIVersion != tC.wantApiVer {
				t.Errorf("ApiVersion: want = %s, got = %s", tC.wantApiVer, res.MinimalConf.APIVersion)
			}
			if res.MinimalConf.Kind != tC.wantKind {
				t.Errorf("Kind: want = %s, got = %s", tC.wantKind, res.MinimalConf.Kind)
			}
			if res.MinimalConf.Spec.DistributionVersion != tC.wantDistroVer {
				t.Errorf(
					"DistributionVersion: want = %s, got = %s",
					tC.wantDistroVer,
					res.MinimalConf.Spec.DistributionVersion,
				)
			}

			if res.DistroManifest.Version != tC.wantDistroVer {
				t.Errorf("ApiVersion: want = %s, got = %s", tC.wantDistroVer, res.DistroManifest.Version)
			}
		})
	}
}
