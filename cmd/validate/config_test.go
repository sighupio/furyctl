// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package validate_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/sighupio/furyctl/cmd/validate"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/yaml"
	"os"
	"path/filepath"
	"testing"
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

	kfdConfig = distribution.Manifest{
		Version: "v1.24.7",
		Modules: struct {
			Auth       string `yaml:"auth"`
			Dr         string `yaml:"dr"`
			Ingress    string `yaml:"ingress"`
			Logging    string `yaml:"logging"`
			Monitoring string `yaml:"monitoring"`
			Opa        string `yaml:"opa"`
		}{},
		Kubernetes: struct {
			Eks struct {
				Version   string `yaml:"version"`
				Installer string `yaml:"installer"`
			} `yaml:"eks"`
		}{},
		FuryctlSchemas: struct {
			Eks []struct {
				ApiVersion string `yaml:"apiVersion"`
				Kind       string `yaml:"kind"`
			} `yaml:"eks"`
		}{},
		Tools: struct {
			Ansible   string `yaml:"ansible"`
			Furyagent string `yaml:"furyagent"`
			Kubectl   string `yaml:"kubectl"`
			Kustomize string `yaml:"kustomize"`
			Terraform string `yaml:"terraform"`
		}{},
	}

	defaults = map[string]interface{}{
		"data": map[string]interface{}{
			"modules": map[string]interface{}{
				"ingress": map[string]interface{}{
					"test": "test",
				},
			},
		},
	}

	failingDefaults = map[string]interface{}{
		"data": map[string]interface{}{
			"modules": map[string]interface{}{
				"ingress": map[string]interface{}{
					"test":       "test",
					"unexpected": "test",
				},
			},
		},
	}

	schema = map[string]interface{}{
		"$schema":              "http://json-schema.org/draft-07/schema#",
		"$id":                  "https://schema.sighup.io/kfd/1.23.2/ekscluster-kfd-v1alpha2.json",
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]interface{}{
			"apiVersion": map[string]interface{}{
				"type": "string",
			},
			"kind": map[string]interface{}{
				"type": "string",
			},
			"spec": map[string]interface{}{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]interface{}{
					"distributionVersion": map[string]interface{}{
						"type": "string",
					},
					"distribution": map[string]interface{}{
						"type":                 "object",
						"additionalProperties": false,
						"properties": map[string]interface{}{
							"modules": map[string]interface{}{
								"type":                 "object",
								"additionalProperties": false,
								"properties": map[string]interface{}{
									"ingress": map[string]interface{}{
										"type":                 "object",
										"additionalProperties": false,
										"properties": map[string]interface{}{
											"test": map[string]interface{}{
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
)

func TestNewConfigCmd_ConfigNotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "furyctl-config-validation-")
	if err != nil {
		t.Fatalf("error creating temp dir: %v", err)
	}

	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	configFilePath := filepath.Join(tmpDir, "furyctl.yaml")

	expectedErr := fmt.Errorf("open %s: no such file or directory", configFilePath)

	b := bytes.NewBufferString("")
	valCmd := validate.NewConfigCmd("dev")

	valCmd.SetOut(b)

	args := []string{"--config", configFilePath}
	valCmd.SetArgs(args)

	err = valCmd.Execute()
	if err != nil && err.Error() != expectedErr.Error() {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
		return
	}
}

func TestNewConfigCmd_WrongDistroLocation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "furyctl-config-validation-")
	if err != nil {
		t.Fatalf("error creating temp dir: %v", err)
	}

	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	configFilePath := filepath.Join(tmpDir, "furyctl.yaml")

	configYaml, err := yaml.MarshalV2(furyConfig)
	if err != nil {
		t.Fatalf("error marshaling config: %v", err)
	}

	if err = os.WriteFile(configFilePath, configYaml, os.ModePerm); err != nil {
		t.Fatalf("error writing config file: %v", err)
	}

	expectedErr := fmt.Errorf("error downloading folder 'wrong-location': downloading options exausted")

	b := bytes.NewBufferString("")
	valCmd := validate.NewConfigCmd("dev")

	valCmd.SetOut(b)

	args := []string{"--config", configFilePath, "--distro-location", "wrong-location"}
	valCmd.SetArgs(args)

	err = valCmd.Execute()
	if err != nil && err.Error() != expectedErr.Error() {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
		return
	}
}

func TestNewConfigCmd_SuccessValidation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "furyctl-config-validation-")
	if err != nil {
		t.Fatalf("error creating temp dir: %v", err)
	}

	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	configFilePath := filepath.Join(tmpDir, "furyctl.yaml")

	configYaml, err := yaml.MarshalV2(furyConfig)
	if err != nil {
		t.Fatalf("error marshaling config: %v", err)
	}

	if err = os.WriteFile(configFilePath, configYaml, os.ModePerm); err != nil {
		t.Fatalf("error writing config file: %v", err)
	}

	kfdFilePath := filepath.Join(tmpDir, "kfd.yaml")

	kfdYaml, err := yaml.MarshalV2(kfdConfig)
	if err != nil {
		t.Fatalf("error marshaling config: %v", err)
	}

	if err = os.WriteFile(kfdFilePath, kfdYaml, os.ModePerm); err != nil {
		t.Fatalf("error writing kfd file: %v", err)
	}

	defaultsFilePath := filepath.Join(tmpDir, "furyctl-defaults.yaml")

	defaultsYaml, err := yaml.MarshalV2(defaults)
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

	schemaJson, err := json.Marshal(schema)
	if err != nil {
		t.Fatalf("error marshaling schema: %v", err)
	}

	if err = os.WriteFile(schemaFilePath, schemaJson, os.ModePerm); err != nil {
		t.Fatalf("error writing schema json: %v", err)
	}

	b := bytes.NewBufferString("")
	valCmd := validate.NewConfigCmd("dev")

	valCmd.SetOut(b)

	args := []string{"--config", configFilePath, "--distro-location", tmpDir}
	valCmd.SetArgs(args)

	err = valCmd.Execute()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
		return
	}
}

func TestNewConfigCmd_FailValidation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "furyctl-config-validation-")
	if err != nil {
		t.Fatalf("error creating temp dir: %v", err)
	}

	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	configFilePath := filepath.Join(tmpDir, "furyctl.yaml")

	configYaml, err := yaml.MarshalV2(furyConfig)
	if err != nil {
		t.Fatalf("error marshaling config: %v", err)
	}

	if err = os.WriteFile(configFilePath, configYaml, os.ModePerm); err != nil {
		t.Fatalf("error writing config file: %v", err)
	}

	kfdFilePath := filepath.Join(tmpDir, "kfd.yaml")

	kfdYaml, err := yaml.MarshalV2(kfdConfig)
	if err != nil {
		t.Fatalf("error marshaling config: %v", err)
	}

	if err = os.WriteFile(kfdFilePath, kfdYaml, os.ModePerm); err != nil {
		t.Fatalf("error writing kfd file: %v", err)
	}

	defaultsFilePath := filepath.Join(tmpDir, "furyctl-defaults.yaml")

	defaultsYaml, err := yaml.MarshalV2(failingDefaults)
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

	schemaJson, err := json.Marshal(schema)
	if err != nil {
		t.Fatalf("error marshaling schema: %v", err)
	}

	if err = os.WriteFile(schemaFilePath, schemaJson, os.ModePerm); err != nil {
		t.Fatalf("error writing schema json: %v", err)
	}

	b := bytes.NewBufferString("")
	valCmd := validate.NewConfigCmd("dev")

	valCmd.SetOut(b)

	args := []string{"--config", configFilePath, "--distro-location", tmpDir}
	valCmd.SetArgs(args)

	expectedError := "furyctl.yaml contains validation errors"

	err = valCmd.Execute()
	if err != nil && err.Error() != expectedError {
		t.Errorf("Expected error %q, got %v", expectedError, err)
		return
	}
}
