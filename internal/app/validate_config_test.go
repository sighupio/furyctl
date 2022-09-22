// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/santhosh-tekuri/jsonschema"

	"github.com/sighupio/furyctl/internal/app"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/yaml"
)

var eksClusterJsonSchema = map[string]any{
	"$schema":              "http://json-schema.org/draft-07/schema#",
	"$id":                  "https://schema.sighup.io/kfd/1.23.2/ekscluster-kfd-v1alpha2.json",
	"type":                 "object",
	"additionalProperties": false,
	"properties": map[string]any{
		"apiVersion": map[string]any{
			"type": "string",
		},
		"kind": map[string]any{
			"type": "string",
		},
		"spec": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]any{
				"distributionVersion": map[string]any{
					"type": "string",
				},
				"distribution": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"properties": map[string]any{
						"modules": map[string]any{
							"type":                 "object",
							"additionalProperties": false,
							"properties": map[string]any{
								"ingress": map[string]any{
									"type":                 "object",
									"additionalProperties": false,
									"properties": map[string]any{
										"test": map[string]any{
											"type": "string",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	},
}

func TestValidateConfig(t *testing.T) {
	testCases := []struct {
		desc                 string
		setup                func(t *testing.T) (string, string)
		teardown             func(t *testing.T, tmpDir string)
		wantErr              bool
		wantErrVal           any
		wantErrType          error
		wantValidationErr    bool
		wantValidationErrVal any
	}{
		{
			desc: "furyctl.yaml not found",
			setup: func(t *testing.T) (string, string) {
				t.Helper()

				tmpDir := mkDirTemp(t, "furyctl-config-validation-")

				configFilePath := filepath.Join(tmpDir, "furyctl.yaml")

				return tmpDir, configFilePath
			},
			teardown: func(t *testing.T, tmpDir string) {
				t.Helper()

				rmDirTemp(t, tmpDir)
			},
			wantErr:    true,
			wantErrVal: &fs.PathError{},
		},
		{
			desc: "wrong distro location",
			setup: func(t *testing.T) (string, string) {
				t.Helper()

				tmpDir := mkDirTemp(t, "furyctl-config-validation-")

				configFilePath := filepath.Join(tmpDir, "furyctl.yaml")

				configYaml, err := yaml.MarshalV2(furyConfig)
				if err != nil {
					t.Fatalf("error marshaling config: %v", err)
				}

				if err := os.WriteFile(configFilePath, configYaml, os.ModePerm); err != nil {
					t.Fatalf("error writing config file: %v", err)
				}

				return "file::/tmp/does-not-exist", configFilePath
			},
			teardown: func(t *testing.T, tmpDir string) {
				t.Helper()

				rmDirTemp(t, tmpDir)
			},
			wantErr:     true,
			wantErrType: distribution.ErrDownloadingFolder,
		},
		{
			desc: "success",
			setup: func(t *testing.T) (string, string) {
				t.Helper()

				return setupDistroFolder(t, correctFuryctlDefaults, correctKFDConf)
			},
			teardown: func(t *testing.T, tmpDir string) {
				t.Helper()

				rmDirTemp(t, tmpDir)
			},
		},
		{
			desc: "failure",
			setup: func(t *testing.T) (string, string) {
				t.Helper()

				return setupDistroFolder(t, wrongFuryctlDefaults, correctKFDConf)
			},
			teardown: func(t *testing.T, tmpDir string) {
				t.Helper()

				rmDirTemp(t, tmpDir)
			},
			wantValidationErr:    true,
			wantValidationErrVal: &jsonschema.ValidationError{},
		},
	}
	for _, tC := range testCases {
		tC := tC

		t.Run(tC.desc, func(t *testing.T) {
			tmpDir, configFilePath := tC.setup(t)

			defer tC.teardown(t, tmpDir)

			vc := app.NewValidateConfig()
			res, err := vc.Execute(app.ValidateConfigRequest{
				FuryctlBinVersion: "unknown",
				DistroLocation:    tmpDir,
				FuryctlConfPath:   configFilePath,
				Debug:             true,
			})

			if tC.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tC.wantErr && err != nil {
				t.Errorf("unexpected error, got = %v", err)
			}

			if tC.wantErrVal != nil && !errors.As(err, &tC.wantErrVal) {
				t.Fatalf("got error = %v, want = %v", err, tC.wantErrVal)
			}
			if tC.wantErrType != nil && !errors.Is(err, tC.wantErrType) {
				t.Fatalf("got error = %v, want = %v", err, tC.wantErrType)
			}

			if tC.wantValidationErr && res.Error == nil {
				t.Fatal("expected validation error, got nil")
			}
			if !tC.wantValidationErr && res.Error != nil {
				t.Fatalf("unexpected validation error, got = %v", res.Error)
			}

			if tC.wantValidationErrVal != nil && !errors.As(res.Error, &tC.wantValidationErrVal) {
				t.Fatalf("got validation error = %v, want = %v", res.Error, tC.wantValidationErrVal)
			}
		})
	}
}
