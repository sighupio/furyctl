package app_test

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/sighupio/furyctl/internal/app"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/execx"
	"github.com/sighupio/furyctl/internal/yaml"
)

var (
	valDepWrongKFDConf = distribution.Manifest{
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
		}{
			Ansible:   "2.10.0",
			Furyagent: "0.0.2",
			Kubectl:   "1.21.4",
			Kustomize: "3.9.8",
			Terraform: "0.15.9",
		},
	}

	valDepCorrectKFDConf = distribution.Manifest{
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
		}{
			Ansible:   "2.11.2",
			Furyagent: "0.0.1",
			Kubectl:   "1.21.1",
			Kustomize: "3.9.4",
			Terraform: "0.15.4",
		},
	}
)

func TestValidateDependencies_MissingToolsAndEnvs(t *testing.T) {
	tmpDir := mkDirTemp(t, "furyctl-deps-validation-")
	defer rmDirTemp(t, tmpDir)

	configFilePath := filepath.Join(tmpDir, "furyctl.yaml")

	configYaml, err := yaml.MarshalV2(furyConfig)
	if err != nil {
		t.Fatalf("error marshaling config: %v", err)
	}

	if err = os.WriteFile(configFilePath, configYaml, os.ModePerm); err != nil {
		t.Fatalf("error writing config file: %v", err)
	}

	kfdFilePath := filepath.Join(tmpDir, "kfd.yaml")

	kfdYaml, err := yaml.MarshalV2(valDepCorrectKFDConf)
	if err != nil {
		t.Fatalf("error marshaling config: %v", err)
	}

	if err = os.WriteFile(kfdFilePath, kfdYaml, os.ModePerm); err != nil {
		t.Fatalf("error writing kfd file: %v", err)
	}

	vd := app.NewValidateDependencies(execx.NewStdExecutor())

	res, err := vd.Execute(app.ValidateDependenciesRequest{
		BinPath:           filepath.Join(tmpDir, "bin"),
		FuryctlBinVersion: "unknown",
		DistroLocation:    tmpDir,
		FuryctlConfPath:   configFilePath,
		Debug:             true,
	})

	if len(res.Errors) == 0 {
		t.Fatal("Expected validation errors, got none")
	}

	if len(res.Errors) != 8 {
		t.Fatalf("Expected 8 validation errors, got %d", len(res.Errors))
	}

	for _, err := range res.Errors {
		var terr *fs.PathError

		if !errors.As(err, &terr) && !errors.Is(err, app.ErrMissingEnvVar) {
			t.Fatalf("Unexpected error: %v", err)
		}
	}
}

func TestValidateDependencies_HasAllToolsAndEnvs(t *testing.T) {
	tmpDir := mkDirTemp(t, "furyctl-deps-validation-")
	defer rmDirTemp(t, tmpDir)

	configFilePath := filepath.Join(tmpDir, "furyctl.yaml")

	configYaml, err := yaml.MarshalV2(furyConfig)
	if err != nil {
		t.Fatalf("error marshaling config: %v", err)
	}

	if err = os.WriteFile(configFilePath, configYaml, os.ModePerm); err != nil {
		t.Fatalf("error writing config file: %v", err)
	}

	kfdFilePath := filepath.Join(tmpDir, "kfd.yaml")

	kfdYaml, err := yaml.MarshalV2(valDepCorrectKFDConf)
	if err != nil {
		t.Fatalf("error marshaling config: %v", err)
	}

	if err = os.WriteFile(kfdFilePath, kfdYaml, os.ModePerm); err != nil {
		t.Fatalf("error writing kfd file: %v", err)
	}

	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	t.Setenv("AWS_DEFAULT_REGION", "test")

	vd := app.NewValidateDependencies(execx.NewFakeExecutor())

	res, err := vd.Execute(app.ValidateDependenciesRequest{
		BinPath:           filepath.Join(tmpDir, "bin"),
		FuryctlBinVersion: "unknown",
		DistroLocation:    tmpDir,
		FuryctlConfPath:   configFilePath,
		Debug:             true,
	})

	if len(res.Errors) != 0 {
		t.Errorf("Did not expect validation errors, got %d", len(res.Errors))
		for _, err := range res.Errors {
			t.Log(err)
		}
	}
}

func TestValidateDependencies_HasWrongTools(t *testing.T) {
	tmpDir := mkDirTemp(t, "furyctl-deps-validation-")
	defer rmDirTemp(t, tmpDir)

	configFilePath := filepath.Join(tmpDir, "furyctl.yaml")

	configYaml, err := yaml.MarshalV2(furyConfig)
	if err != nil {
		t.Fatalf("error marshaling config: %v", err)
	}

	if err = os.WriteFile(configFilePath, configYaml, os.ModePerm); err != nil {
		t.Fatalf("error writing config file: %v", err)
	}

	kfdFilePath := filepath.Join(tmpDir, "kfd.yaml")

	kfdYaml, err := yaml.MarshalV2(valDepWrongKFDConf)
	if err != nil {
		t.Fatalf("error marshaling config: %v", err)
	}

	if err = os.WriteFile(kfdFilePath, kfdYaml, os.ModePerm); err != nil {
		t.Fatalf("error writing kfd file: %v", err)
	}

	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	t.Setenv("AWS_DEFAULT_REGION", "test")

	vd := app.NewValidateDependencies(execx.NewFakeExecutor())

	res, err := vd.Execute(app.ValidateDependenciesRequest{
		BinPath:           filepath.Join(tmpDir, "bin"),
		FuryctlBinVersion: "unknown",
		DistroLocation:    tmpDir,
		FuryctlConfPath:   configFilePath,
		Debug:             true,
	})

	if len(res.Errors) != 5 {
		t.Fatalf("Expected 5 validation errors, got %d", len(res.Errors))
	}

	for _, err := range res.Errors {
		if !errors.Is(err, app.ErrWrongToolVersion) {
			t.Errorf("Unexpected error: %v", err)
		}
	}
}

func TestHelperProcess(t *testing.T) {
	args := os.Args

	if len(args) < 3 || args[1] != "-test.run=TestHelperProcess" {
		return
	}

	cmd, _ := args[3], args[4:]

	switch cmd {
	case "ansible":
		fmt.Fprintf(os.Stdout, "ansible [core 2.11.2]\n  "+
			"config file = None\n  "+
			"configured module search path = ['', '']\n"+
			"ansible python module location = ./ansible\n"+
			"ansible collection location = ./ansible/collections\n"+
			"executable location = ./bin/ansible\n  "+
			"python version = 3.9.14\n"+
			"jinja version = 3.1.2\n"+
			"libyaml = True\n")
	case "terraform":
		fmt.Fprintf(os.Stdout, "Terraform v0.15.4\non darwin_amd64")
	case "kubectl":
		fmt.Fprintf(os.Stdout, "Client Version: version.Info{Major:\"1\", "+
			"Minor:\"21\", GitVersion:\"v1.21.1\", GitCommit:\"xxxxx\", "+
			"GitTreeState:\"clean\", BuildDate:\"2021-05-12T14:00:00Z\", "+
			"GoVersion:\"go1.16.4\", Compiler:\"gc\", Platform:\"darwin/amd64\"}\n")
	case "kustomize":
		fmt.Fprintf(os.Stdout, "Version: {kustomize/v3.9.4 GitCommit:xxxxxxx"+
			"BuildDate:2021-05-12T14:00:00Z GoOs:darwin GoArch:amd64}")
	case "furyagent":
		fmt.Fprintf(os.Stdout, "furyagent version 0.0.1")
	default:
		fmt.Fprintf(os.Stdout, "command not found")
	}

	os.Exit(0)
}
