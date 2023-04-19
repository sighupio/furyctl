// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package distribution_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/sighupio/fury-distribution/pkg/config"
	"github.com/sighupio/furyctl/internal/distribution"
)

func TestGetTemplatePath(t *testing.T) {
	tests := []struct {
		name     string
		basePath string
		conf     config.Furyctl
		want     string
		wantErr  error
	}{
		{
			name:     "test with base path",
			basePath: "testpath",
			conf: config.Furyctl{
				APIVersion: "kfd.sighup.io/v1alpha2",
				Kind:       "EKSCluster",
				Spec:       config.FuryctlSpec{},
			},
			want: fmt.Sprintf("%s", filepath.Join(
				"testpath",
				"templates/config",
				"ekscluster-kfd-v1alpha2.yaml.tpl",
			)),
			wantErr: nil,
		},
		{
			name:     "test without base path",
			basePath: "",
			conf: config.Furyctl{
				APIVersion: "kfd.sighup.io/v1alpha2",
				Kind:       "EKSCluster",
				Spec:       config.FuryctlSpec{},
			},
			want:    fmt.Sprintf("%s", filepath.Join("templates/config", "ekscluster-kfd-v1alpha2.yaml.tpl")),
			wantErr: nil,
		},
		{
			name:     "test with invalid apiVersion",
			basePath: "",
			conf: config.Furyctl{
				APIVersion: "",
				Kind:       "EKSCluster",
				Spec:       config.FuryctlSpec{},
			},
			want:    "",
			wantErr: fmt.Errorf("invalid apiVersion: "),
		},
		{
			name:     "test with invalid kind",
			basePath: "",
			conf: config.Furyctl{
				APIVersion: "kfd.sighup.io/v1alpha2",
				Kind:       "",
				Spec:       config.FuryctlSpec{},
			},
			want:    "",
			wantErr: fmt.Errorf("kind is empty"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := distribution.GetConfigTemplatePath(tt.basePath, tt.conf)
			if err != nil {
				if err.Error() != tt.wantErr.Error() {
					t.Errorf("distribution.GetTemplatePath() error = %v, wantErr %v", err, tt.wantErr)
				}

				return
			}

			if got != tt.want {
				t.Errorf("distribution.GetTemplatePath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetSchemaPaths(t *testing.T) {
	verifyPaths := func(t *testing.T, fname, got, want string, err, wantErr error) {
		if err != nil {
			if err.Error() != wantErr.Error() {
				t.Errorf("distribution.%s() error = %v, wantErr %v", fname, err, wantErr)
			}

			return
		}

		if got != want {
			t.Errorf("distribution.%s() = %v, want %v", fname, got, want)
		}
	}

	tests := []struct {
		name     string
		basePath string
		conf     config.Furyctl
		want     string
		wantErr  error
	}{
		{
			name:     "test with base path",
			basePath: "testpath",
			conf: config.Furyctl{
				APIVersion: "kfd.sighup.io/v1alpha2",
				Kind:       "EKSCluster",
				Spec:       config.FuryctlSpec{},
			},
			want: filepath.Join(
				"testpath",
				"schemas",
				"%s",
				"ekscluster-kfd-v1alpha2.json",
			),
			wantErr: nil,
		},
		{
			name:     "test without base path",
			basePath: "",
			conf: config.Furyctl{
				APIVersion: "kfd.sighup.io/v1alpha2",
				Kind:       "EKSCluster",
				Spec:       config.FuryctlSpec{},
			},
			want: filepath.Join(
				"schemas",
				"%s",
				"ekscluster-kfd-v1alpha2.json",
			),
			wantErr: nil,
		},
		{
			name:     "test with invalid apiVersion",
			basePath: "",
			conf: config.Furyctl{
				APIVersion: "",
				Kind:       "EKSCluster",
				Spec:       config.FuryctlSpec{},
			},
			want:    "",
			wantErr: fmt.Errorf("invalid apiVersion: "),
		},
		{
			name:     "test with invalid kind",
			basePath: "",
			conf: config.Furyctl{
				APIVersion: "kfd.sighup.io/v1alpha2",
				Kind:       "",
				Spec:       config.FuryctlSpec{},
			},
			want:    "",
			wantErr: fmt.Errorf("kind is empty"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			egot, eerr := distribution.GetPublicSchemaPath(tt.basePath, tt.conf)
			verifyPaths(t, "GetPublicSchemaPath", egot, fmt.Sprintf(tt.want, "public"), eerr, tt.wantErr)

			igot, ierr := distribution.GetPrivateSchemaPath(tt.basePath, tt.conf)
			verifyPaths(t, "GetPrivateSchemaPath", igot, fmt.Sprintf(tt.want, "private"), ierr, tt.wantErr)
		})
	}
}

func TestGetDefaultsPath(t *testing.T) {
	dp := distribution.GetDefaultsPath("/tmp")

	if dp != "/tmp/furyctl-defaults.yaml" {
		t.Errorf("expected /tmp/furyctl-defaults.yaml, got %s", dp)
	}
}
