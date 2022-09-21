package app_test

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/sighupio/furyctl/internal/app"
	"github.com/sighupio/furyctl/internal/app/validate"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/yaml"
)

var (
	valConfKFDConf = distribution.Manifest{
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

	valConfCorrectDefaults = map[string]interface{}{
		"data": map[string]interface{}{
			"modules": map[string]interface{}{
				"ingress": map[string]interface{}{
					"test": "test",
				},
			},
		},
	}

	valConfWrongDefaults = map[string]interface{}{
		"data": map[string]interface{}{
			"modules": map[string]interface{}{
				"ingress": map[string]interface{}{
					"test":       "test",
					"unexpected": "test",
				},
			},
		},
	}

	valConfSchema = map[string]interface{}{
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

func TestValidateConfig_FuryctlConfigNotFound(t *testing.T) {
	tmpDir := mkDirTemp(t, "furyctl-config-validation-")
	defer rmDirTemp(t, tmpDir)

	configFilePath := filepath.Join(tmpDir, "furyctl.yaml")

	vc := app.NewValidateConfig()
	_, err := vc.Execute(app.ValidateConfigRequest{
		FuryctlBinVersion: "unknown",
		DistroLocation:    tmpDir,
		FuryctlConfPath:   configFilePath,
		Debug:             true,
	})

	if err == nil {
		t.Error("Expected error, got none")
	}

	var terr *fs.PathError

	if !errors.As(err, &terr) {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestValidateConfig_WrongDistroLocation(t *testing.T) {
	tmpDir := mkDirTemp(t, "furyctl-config-validation-")
	defer rmDirTemp(t, tmpDir)

	configFilePath := filepath.Join(tmpDir, "furyctl.yaml")

	configYaml, err := yaml.MarshalV2(furyConfig)
	if err != nil {
		t.Fatalf("error marshaling config: %v", err)
	}

	if err := os.WriteFile(configFilePath, configYaml, os.ModePerm); err != nil {
		t.Fatalf("error writing config file: %v", err)
	}

	vc := app.NewValidateConfig()
	_, err = vc.Execute(app.ValidateConfigRequest{
		FuryctlBinVersion: "unknown",
		DistroLocation:    "file::/tmp/does-not-exist",
		FuryctlConfPath:   configFilePath,
		Debug:             true,
	})

	if err == nil {
		t.Error("Expected error, got none")
	}

	if !errors.Is(err, validate.ErrDownloadingFolder) {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestValidateConfig_Success(t *testing.T) {
	tmpDir := mkDirTemp(t, "furyctl-config-validation-")
	defer rmDirTemp(t, tmpDir)

	configFilePath := filepath.Join(tmpDir, "furyctl.yaml")

	configYaml, err := yaml.MarshalV2(furyConfig)
	if err != nil {
		t.Fatalf("error marshaling config: %v", err)
	}

	if err := os.WriteFile(configFilePath, configYaml, os.ModePerm); err != nil {
		t.Fatalf("error writing config file: %v", err)
	}

	kfdFilePath := filepath.Join(tmpDir, "kfd.yaml")

	kfdYaml, err := yaml.MarshalV2(valConfKFDConf)
	if err != nil {
		t.Fatalf("error marshaling config: %v", err)
	}

	if err = os.WriteFile(kfdFilePath, kfdYaml, os.ModePerm); err != nil {
		t.Fatalf("error writing kfd file: %v", err)
	}

	defaultsFilePath := filepath.Join(tmpDir, "furyctl-defaults.yaml")

	defaultsYaml, err := yaml.MarshalV2(valConfCorrectDefaults)
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

	schemaJson, err := json.Marshal(valConfSchema)
	if err != nil {
		t.Fatalf("error marshaling schema: %v", err)
	}

	if err = os.WriteFile(schemaFilePath, schemaJson, os.ModePerm); err != nil {
		t.Fatalf("error writing schema json: %v", err)
	}

	vc := app.NewValidateConfig()
	res, err := vc.Execute(app.ValidateConfigRequest{
		FuryctlBinVersion: "unknown",
		DistroLocation:    tmpDir,
		FuryctlConfPath:   configFilePath,
		Debug:             true,
	})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if res.Error != nil {
		t.Fatalf("Unexpected validation error: %v", res.Error)
	}
}

func TestValidateConfig_Failure(t *testing.T) {
	tmpDir := mkDirTemp(t, "furyctl-config-validation-")
	defer rmDirTemp(t, tmpDir)

	configFilePath := filepath.Join(tmpDir, "furyctl.yaml")

	configYaml, err := yaml.MarshalV2(furyConfig)
	if err != nil {
		t.Fatalf("error marshaling config: %v", err)
	}

	if err := os.WriteFile(configFilePath, configYaml, os.ModePerm); err != nil {
		t.Fatalf("error writing config file: %v", err)
	}

	kfdFilePath := filepath.Join(tmpDir, "kfd.yaml")

	kfdYaml, err := yaml.MarshalV2(valConfKFDConf)
	if err != nil {
		t.Fatalf("error marshaling config: %v", err)
	}

	if err = os.WriteFile(kfdFilePath, kfdYaml, os.ModePerm); err != nil {
		t.Fatalf("error writing kfd file: %v", err)
	}

	defaultsFilePath := filepath.Join(tmpDir, "furyctl-defaults.yaml")

	defaultsYaml, err := yaml.MarshalV2(valConfWrongDefaults)
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

	schemaJson, err := json.Marshal(valConfSchema)
	if err != nil {
		t.Fatalf("error marshaling schema: %v", err)
	}

	if err = os.WriteFile(schemaFilePath, schemaJson, os.ModePerm); err != nil {
		t.Fatalf("error writing schema json: %v", err)
	}

	vc := app.NewValidateConfig()
	res, err := vc.Execute(app.ValidateConfigRequest{
		FuryctlBinVersion: "unknown",
		DistroLocation:    tmpDir,
		FuryctlConfPath:   configFilePath,
		Debug:             true,
	})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if res.Error == nil {
		t.Fatalf("Expected validation errors, got none")
	}
}
