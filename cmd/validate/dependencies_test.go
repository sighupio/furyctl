// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package validate

import (
	"bytes"
	"fmt"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/yaml"
	"os"
	"os/exec"
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

	kfdConfigCorrect = distribution.Manifest{
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

	kfdConfigWrong = distribution.Manifest{
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
)

func TestNewDependenciesCmd_FailSysDepsValidation(t *testing.T) {
	execCommand = fakeExecCommand

	defer func() {
		execCommand = exec.Command
	}()

	tmpDir, err := os.MkdirTemp("", "furyctl-deps-validation-")
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

	kfdYaml, err := yaml.MarshalV2(kfdConfigWrong)
	if err != nil {
		t.Fatalf("error marshaling config: %v", err)
	}

	if err = os.WriteFile(kfdFilePath, kfdYaml, os.ModePerm); err != nil {
		t.Fatalf("error writing kfd file: %v", err)
	}

	b := bytes.NewBufferString("")
	valCmd := NewDependenciesCmd("dev")

	valCmd.SetOut(b)

	args := []string{"--config", configFilePath, "--distro-location", tmpDir}
	valCmd.SetArgs(args)

	err = valCmd.Execute()
	if err != ErrSystemDepsValidation {
		t.Fatalf("want: %v, got: %v", ErrSystemDepsValidation, err)
	}
}

func TestNewDependenciesCmd_FailEnvVarsValidation(t *testing.T) {
	execCommand = fakeExecCommand

	defer func() {
		execCommand = exec.Command
	}()

	tmpDir, err := os.MkdirTemp("", "furyctl-deps-validation-")
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

	kfdYaml, err := yaml.MarshalV2(kfdConfigCorrect)
	if err != nil {
		t.Fatalf("error marshaling config: %v", err)
	}

	if err = os.WriteFile(kfdFilePath, kfdYaml, os.ModePerm); err != nil {
		t.Fatalf("error writing kfd file: %v", err)
	}

	b := bytes.NewBufferString("")
	valCmd := NewDependenciesCmd("dev")

	valCmd.SetOut(b)

	args := []string{"--config", configFilePath, "--distro-location", tmpDir}
	valCmd.SetArgs(args)

	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	t.Setenv("AWS_DEFAULT_REGION", "")

	err = valCmd.Execute()
	if err != ErrEnvironmentDepsValidation {
		t.Fatalf("want: %v, got: %v", ErrEnvironmentDepsValidation, err)
	}
}

func TestNewDependenciesCmd_SuccessValidation(t *testing.T) {
	execCommand = fakeExecCommand

	defer func() {
		execCommand = exec.Command
	}()

	tmpDir, err := os.MkdirTemp("", "furyctl-deps-validation-")
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

	kfdYaml, err := yaml.MarshalV2(kfdConfigCorrect)
	if err != nil {
		t.Fatalf("error marshaling config: %v", err)
	}

	if err = os.WriteFile(kfdFilePath, kfdYaml, os.ModePerm); err != nil {
		t.Fatalf("error writing kfd file: %v", err)
	}

	b := bytes.NewBufferString("")
	valCmd := NewDependenciesCmd("dev")

	valCmd.SetOut(b)

	args := []string{"--config", configFilePath, "--distro-location", tmpDir}
	valCmd.SetArgs(args)

	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	t.Setenv("AWS_DEFAULT_REGION", "test")

	err = valCmd.Execute()
	if err != nil {
		t.Fatalf("want: %v, got: %v", nil, err)
	}
}

func TestValidateEnvDependencies(t *testing.T) {
	tests := []struct {
		name    string
		kind    string
		setup   func()
		wantErr error
	}{
		{
			name: "test with correct dependencies: eks",
			kind: "EKSCluster",
			setup: func() {
				t.Setenv("AWS_ACCESS_KEY_ID", "test")
				t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
				t.Setenv("AWS_DEFAULT_REGION", "test")
			},
			wantErr: nil,
		},
		{
			name: "test with incorrect dependencies: eks",
			kind: "EKSCluster",
			setup: func() {
				t.Setenv("AWS_ACCESS_KEY_ID", "")
				t.Setenv("AWS_SECRET_ACCESS_KEY", "")
				t.Setenv("AWS_DEFAULT_REGION", "")
			},
			wantErr: ErrEnvironmentDepsValidation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			err := validateEnvDependencies(tt.kind)
			if err != nil {
				if tt.wantErr == nil {
					t.Errorf("validateEnvDependencies() error = %v, wantErr %v", err, tt.wantErr)
					return
				}

				if err.Error() != tt.wantErr.Error() {
					t.Errorf("validateEnvDependencies() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
			}

			if err == nil && tt.wantErr != nil {
				t.Errorf("validateEnvDependencies() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestValidateSystemDependencies(t *testing.T) {
	execCommand = fakeExecCommand
	defer func() { execCommand = exec.Command }()

	tests := []struct {
		name     string
		manifest distribution.Manifest
		wantErr  error
	}{
		{
			name:     "test with correct dependencies",
			manifest: kfdConfigCorrect,
			wantErr:  nil,
		},
		{
			name:     "test with incorrect dependencies",
			manifest: kfdConfigWrong,
			wantErr:  ErrSystemDepsValidation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSystemDependencies(tt.manifest)
			if err != nil {
				if tt.wantErr == nil {
					t.Errorf("validateSystemDependencies() error = %v, wantErr %v", err, tt.wantErr)
					return
				}

				if err.Error() != tt.wantErr.Error() {
					t.Errorf("validateSystemDependencies() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
			}

			if err == nil && tt.wantErr != nil {
				t.Errorf("validateSystemDependencies() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestCheckAnsibleVersion(t *testing.T) {
	execCommand = fakeExecCommand
	defer func() { execCommand = exec.Command }()

	tests := []struct {
		name    string
		version string
		want    string
		wantErr error
	}{
		{
			name:    "test with correct ansible version",
			version: "2.11.2",
			want:    "",
			wantErr: nil,
		},
		{
			name:    "test with incorrect ansible version",
			version: "2.10.6",
			want:    "",
			wantErr: fmt.Errorf("ansible version on system: 2.11.2, required version: 2.10.6"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkAnsibleVersion(tt.version)
			if err != nil {
				if tt.wantErr == nil {
					t.Errorf("checkAnsibleVersion() error = %v, wantErr %v", err, tt.wantErr)
					return
				}

				if err.Error() != tt.wantErr.Error() {
					t.Errorf("checkAnsibleVersion() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
			}

			if err == nil && tt.wantErr != nil {
				t.Errorf("checkAnsibleVersion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestCheckTerraformVersion(t *testing.T) {
	execCommand = fakeExecCommand
	defer func() { execCommand = exec.Command }()

	tests := []struct {
		name    string
		version string
		want    string
		wantErr error
	}{
		{
			name:    "test with correct terraform version",
			version: "0.15.4",
			want:    "",
			wantErr: nil,
		},
		{
			name:    "test with incorrect terraform version",
			version: "0.14.6",
			want:    "",
			wantErr: fmt.Errorf("terraform version on system: 0.15.4, required version: 0.14.6"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkTerraformVersion(tt.version)
			if err != nil {
				if tt.wantErr == nil {
					t.Errorf("checkTerraformVersion() error = %v, wantErr %v", err, tt.wantErr)
					return
				}

				if err.Error() != tt.wantErr.Error() {
					t.Errorf("checkTerraformVersion() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
			}

			if err == nil && tt.wantErr != nil {
				t.Errorf("checkTerraformVersion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestCheckKubectlVersion(t *testing.T) {
	execCommand = fakeExecCommand
	defer func() { execCommand = exec.Command }()

	tests := []struct {
		name    string
		version string
		want    string
		wantErr error
	}{
		{
			name:    "test with correct kubectl version",
			version: "1.21.1",
			want:    "",
			wantErr: nil,
		},
		{
			name:    "test with incorrect kubectl version",
			version: "1.21.2",
			want:    "",
			wantErr: fmt.Errorf("kubectl version on system: 1.21.1, required version: 1.21.2"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkKubectlVersion(tt.version)
			if err != nil {
				if tt.wantErr == nil {
					t.Errorf("checkKubectlVersion() error = %v, wantErr %v", err, tt.wantErr)
					return
				}

				if err.Error() != tt.wantErr.Error() {
					t.Errorf("checkKubectlVersion() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
			}

			if err == nil && tt.wantErr != nil {
				t.Errorf("checkKubectlVersion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestCheckKustomizeVersion(t *testing.T) {
	execCommand = fakeExecCommand
	defer func() { execCommand = exec.Command }()

	tests := []struct {
		name    string
		version string
		want    string
		wantErr error
	}{
		{
			name:    "test with correct kustomize version",
			version: "3.9.4",
			want:    "",
			wantErr: nil,
		},
		{
			name:    "test with incorrect kustomize version",
			version: "3.9.3",
			want:    "",
			wantErr: fmt.Errorf("kustomize version on system: 3.9.4, required version: 3.9.3"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkKustomizeVersion(tt.version)
			if err != nil {
				if tt.wantErr == nil {
					t.Errorf("checkKustomizeVersion() error = %v, wantErr %v", err, tt.wantErr)
					return
				}

				if err.Error() != tt.wantErr.Error() {
					t.Errorf("checkKustomizeVersion() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
			}

			if err == nil && tt.wantErr != nil {
				t.Errorf("checkKustomizeVersion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestCheckFuryagentVersion(t *testing.T) {
	execCommand = fakeExecCommand
	defer func() { execCommand = exec.Command }()

	tests := []struct {
		name    string
		version string
		want    string
		wantErr error
	}{
		{
			name:    "test with correct furyagent version",
			version: "0.0.1",
			want:    "",
			wantErr: nil,
		},
		{
			name:    "test with incorrect furyagent version",
			version: "0.0.2",
			want:    "",
			wantErr: fmt.Errorf("furyagent version on system: 0.0.1, required version: 0.0.2"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkFuryagentVersion(tt.version)
			if err != nil {
				if tt.wantErr == nil {
					t.Errorf("checkFuryagentVersion() error = %v, wantErr %v", err, tt.wantErr)
					return
				}

				if err.Error() != tt.wantErr.Error() {
					t.Errorf("checkFuryagentVersion() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
			}

			if err == nil && tt.wantErr != nil {
				t.Errorf("checkFuryagentVersion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func fakeExecCommand(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	return cmd
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
