// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/semver"
)

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
			got, err := getSchemaPath(tt.basePath, tt.conf)
			if err != nil {
				if err.Error() != tt.wantErr.Error() {
					t.Errorf("getSchemaPath() error = %v, wantErr %v", err, tt.wantErr)
				}

				return
			}

			if got != tt.want {
				t.Errorf("getSchemaPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDownloadDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "furyctl-download-test-")
	if err != nil {
		t.Fatalf("error creating temp dir: %v", err)
	}

	src, err := os.Create(filepath.Join(tmpDir, "test.txt"))
	if err != nil {
		t.Fatalf("error creating temp input file: %v", err)
	}

	defer func() {
		src.Close()
		_ = os.RemoveAll(tmpDir)
	}()

	dlDir, err := downloadDirectory(tmpDir)
	if err != nil {
		t.Fatalf("error downloading directory: %v", err)
	}

	_, err = os.Stat(filepath.Join(dlDir, "test.txt"))
	if err != nil {
		t.Fatalf("error checking downloaded file: %v", err)
	}
}

func TestClientGet(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "furyctl-clientget-test-")
	if err != nil {
		t.Fatalf("error creating temp dir: %v", err)
	}

	in := filepath.Join(tmpDir, "in")
	out := filepath.Join(tmpDir, "out")

	if err := os.MkdirAll(in, 0755); err != nil {
		t.Fatalf("error creating temp dir: %v", err)
	}

	src, err := os.Create(filepath.Join(in, "test.txt"))
	if err != nil {
		t.Fatalf("error creating temp input file: %v", err)
	}

	defer func() {
		src.Close()
		_ = os.RemoveAll(tmpDir)
	}()

	err = clientGet(in, out)
	if err != nil {
		t.Fatalf("error getting directory: %v", err)
	}

	_, err = os.Stat(filepath.Join(out, "test.txt"))
	if err != nil {
		t.Fatalf("error getting file: %v", err)
	}
}

func TestUrlHasForcedProtocol(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{
			name: "test with http",
			url:  "http::test.com",
			want: true,
		},
		{
			name: "test without protocol",
			url:  "test.com",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := urlHasForcedProtocol(tt.url); got != tt.want {
				t.Errorf("urlHasForcedProtocol() = %v, want %v", got, tt.want)
			}
		})
	}
}
