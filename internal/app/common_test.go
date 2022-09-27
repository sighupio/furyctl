// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/sighupio/fury-distribution/pkg/config"
	"github.com/sighupio/furyctl/internal/yaml"
)

var (
	furyConfig = map[string]interface{}{
		"apiVersion": "kfd.sighup.io/v1alpha2",
		"kind":       "EKSCluster",
		"spec": map[string]interface{}{
			"distributionVersion": "v1.24.7",
			"distribution":        map[string]interface{}{},
		},
	}

	correctFuryctlDefaults = map[string]any{
		"data": map[string]any{
			"modules": map[string]any{
				"ingress": map[string]any{
					"test": "test",
				},
			},
		},
	}

	wrongFuryctlDefaults = map[string]any{
		"data": map[string]any{
			"modules": map[string]any{
				"ingress": map[string]any{
					"test":       "test",
					"unexpected": "test",
				},
			},
		},
	}

	correctKFDConf = config.KFD{
		Version: "v1.24.7",
		Modules: config.KFDModules{
			Auth:       "v0.0.1",
			Dr:         "v1.9.3",
			Ingress:    "v1.12.2",
			Logging:    "v2.0.3",
			Monitoring: "v1.14.2",
			Opa:        "v1.7.0",
		},
		Kubernetes: config.KFDKubernetes{
			Eks: config.KFDProvider{
				Version:   "1.23",
				Installer: "v1.9.1",
			},
		},
		FuryctlSchemas: config.KFDSchemas{
			Eks: []config.KFDSchema{
				{
					ApiVersion: "kfd.sighup.io/v1alpha2",
					Kind:       "EKSCluster",
				},
			},
		},
		Tools: config.KFDTools{
			Ansible:   "2.11.2",
			Furyagent: "0.3.0",
			Kubectl:   "1.21.1",
			Kustomize: "3.9.4",
			Terraform: "0.15.4",
		},
	}

	wrongToolsKFDConf = config.KFD{
		Version: "v1.24.7",
		Modules: config.KFDModules{
			Auth:       "v0.0.1",
			Dr:         "v1.9.3",
			Ingress:    "v1.12.2",
			Logging:    "v2.0.3",
			Monitoring: "v1.14.2",
			Opa:        "v1.7.0",
		},
		Kubernetes: config.KFDKubernetes{
			Eks: config.KFDProvider{
				Version:   "1.23",
				Installer: "v1.9.1",
			},
		},
		FuryctlSchemas: config.KFDSchemas{
			Eks: []config.KFDSchema{
				{
					ApiVersion: "kfd.sighup.io/v1alpha2",
					Kind:       "EKSCluster",
				},
			},
		},
		Tools: config.KFDTools{
			Ansible:   "2.10.0",
			Furyagent: "0.0.2",
			Kubectl:   "1.21.4",
			Kustomize: "3.9.8",
			Terraform: "0.15.9",
		},
	}
)

func mkDirTemp(t *testing.T, prefix string) string {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", prefix)
	if err != nil {
		t.Fatalf("error creating temp dir: %v", err)
	}

	return tmpDir
}

func rmDirTemp(t *testing.T, dir string) {
	t.Helper()

	if err := os.RemoveAll(dir); err != nil {
		t.Log(err)
	}
}

func setupDistroFolder(t *testing.T, furyctlDefaults map[string]any, kfdConf config.KFD) (string, string) {
	t.Helper()

	tmpDir := mkDirTemp(t, "furyctl-validate-test-")

	configFilePath := filepath.Join(tmpDir, "furyctl.yaml")

	configYaml, err := yaml.MarshalV2(furyConfig)
	if err != nil {
		t.Fatalf("error marshaling config: %v", err)
	}

	if err := os.WriteFile(configFilePath, configYaml, os.ModePerm); err != nil {
		t.Fatalf("error writing config file: %v", err)
	}

	kfdFilePath := filepath.Join(tmpDir, "kfd.yaml")

	kfdYaml, err := yaml.MarshalV2(kfdConf)
	if err != nil {
		t.Fatalf("error marshaling config: %v", err)
	}

	if err = os.WriteFile(kfdFilePath, kfdYaml, os.ModePerm); err != nil {
		t.Fatalf("error writing kfd file: %v", err)
	}

	defaultsFilePath := filepath.Join(tmpDir, "furyctl-defaults.yaml")

	defaultsYaml, err := yaml.MarshalV2(furyctlDefaults)
	if err != nil {
		t.Fatalf("error marshaling furyctl defaults: %v", err)
	}

	if err = os.WriteFile(defaultsFilePath, defaultsYaml, os.ModePerm); err != nil {
		t.Fatalf("error writing furyctl defaults: %v", err)
	}

	schemasPath := filepath.Join(tmpDir, "schemas")

	err = os.Mkdir(schemasPath, os.ModePerm)
	if err != nil {
		t.Fatalf("error creating schemas dir: %v", err)
	}

	schemaFilePath := filepath.Join(schemasPath, "ekscluster-kfd-v1alpha2.json")

	schemaJson, err := json.Marshal(eksClusterJsonSchema)
	if err != nil {
		t.Fatalf("error marshaling schema: %v", err)
	}

	if err = os.WriteFile(schemaFilePath, schemaJson, os.ModePerm); err != nil {
		t.Fatalf("error writing schema json: %v", err)
	}

	return tmpDir, configFilePath
}
