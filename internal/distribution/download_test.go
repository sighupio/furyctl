// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package distribution_test

import (
	"fmt"
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
