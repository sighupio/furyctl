// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build integration

package app_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sighupio/furyctl/internal/app"
	"github.com/sighupio/furyctl/internal/netx"
)

func TestDownloadDependencies(t *testing.T) {
	testCases := []struct {
		desc        string
		setup       func(t *testing.T) (string, string)
		teardown    func(t *testing.T, tmpDir string)
		wantErr     bool
		wantDepsErr bool
		wantFiles   []string
	}{
		{
			desc:        "success",
			wantErr:     false,
			wantDepsErr: false,
			wantFiles: []string{
				"vendor/bin/furyagent",
				"vendor/bin/kubectl",
				"vendor/bin/kustomize",
				"vendor/bin/terraform",
				"vendor/installers/eks/README.md",
				"vendor/installers/eks/modules/eks/main.tf",
				"vendor/installers/eks/modules/vpc-and-vpn/main.tf",
				"vendor/modules/auth/README.md",
				"vendor/modules/auth/katalog/gangway/kustomization.yaml",
				"vendor/modules/dr/README.md",
				"vendor/modules/dr/katalog/velero/velero-aws/kustomization.yaml",
				"vendor/modules/ingress/README.md",
				"vendor/modules/ingress/katalog/nginx/kustomization.yaml",
				"vendor/modules/logging/README.md",
				"vendor/modules/logging/katalog/configs/kustomization.yaml",
				"vendor/modules/monitoring/README.md",
				"vendor/modules/monitoring/katalog/configs/kustomization.yaml",
				"vendor/modules/opa/README.md",
				"vendor/modules/opa/katalog/gatekeeper/kustomization.yaml",
			},
		},
	}
	for _, tC := range testCases {
		tC := tC

		t.Run(tC.desc, func(t *testing.T) {
			basePath, err := os.MkdirTemp("", "furyctl-test-")
			if err != nil {
				t.Fatalf("error creating tmp dir for test: %v", err)
			}
			defer os.RemoveAll(basePath)

			t.Logf("basePath: %s", basePath)

			distroLocation, err := filepath.Abs("../../test/data/v1.23.3/distro")
			if err != nil {
				t.Fatal(err)
			}

			vc := app.NewDownloadDependencies(netx.NewGoGetterClient(), basePath)

			res, err := vc.Execute(app.DownloadDependenciesRequest{
				FuryctlBinVersion: "unknown",
				DistroLocation:    distroLocation,
				FuryctlConfPath:   "../../test/data/v1.23.3/furyctl.yaml",
				Debug:             true,
			})

			if tC.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tC.wantErr && err != nil {
				t.Errorf("unexpected error, got = %v", err)
			}

			if tC.wantDepsErr && len(res.DepsErrors) == 0 {
				t.Fatal("expected deps download error, got none")
			}
			if !tC.wantDepsErr && len(res.DepsErrors) != 0 {
				t.Fatalf("unexpected deps download error, got = %v", res.DepsErrors)
			}

			for _, f := range tC.wantFiles {
				info, err := os.Stat(filepath.Join(basePath, f))
				if err != nil {
					if os.IsNotExist(err) {
						t.Errorf("expected file %s to exist, but it doesn't", f)
					} else {
						t.Fatalf("unexpected error with file '%s': %v", f, err)
					}

					continue
				}

				if info.IsDir() {
					t.Errorf("expected '%s' to be a file, it's a directory instead", f)
				}
			}
		})
	}
}
