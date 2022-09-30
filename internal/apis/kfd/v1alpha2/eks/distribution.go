// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"

	"github.com/sighupio/fury-distribution/pkg/config"
	"github.com/sighupio/furyctl/internal/template"
	"github.com/sighupio/furyctl/internal/yaml"
)

type Distribution struct {
	Path          string
	TerraformPath string
	KustomizePath string
	PlanPath      string
	LogsPath      string
	OutputsPath   string
	SecretsPath   string
}

func NewDistribution() (*Distribution, error) {
	infraPath := path.Join(".distribution")

	binPath, err := filepath.Abs("./vendor")
	if err != nil {
		return &Distribution{}, err
	}

	terraformPath := path.Join(binPath, "bin", "terraform")
	kustomizePath := path.Join(binPath, "bin", "kustomize")

	planPath := path.Join(infraPath, "terraform", "plan")
	logsPath := path.Join(infraPath, "terraform", "logs")
	outputsPath := path.Join(infraPath, "terraform", "outputs")
	secretsPath := path.Join(infraPath, "terraform", "secrets")

	return &Distribution{
		Path:          infraPath,
		TerraformPath: terraformPath,
		KustomizePath: kustomizePath,
		PlanPath:      planPath,
		LogsPath:      logsPath,
		OutputsPath:   outputsPath,
		SecretsPath:   secretsPath,
	}, nil
}

func (i *Distribution) CreateFolder() error {
	return os.Mkdir(i.Path, 0o755)
}

func (i *Distribution) CopyFromTemplate(kfdManifest config.KFD) error {
	var config template.Config

	sourceTfDir := path.Join("configs", "provisioners", "bootstrap", "aws")
	targetTfDir := path.Join(i.Path, "terraform")

	outDirPath, err := os.MkdirTemp("", "furyctl-dist-")
	if err != nil {
		return err
	}

	tfConfVars := map[string]map[any]any{
		"kubernetes": {
			"eks": kfdManifest.Kubernetes.Eks,
		},
	}

	config.Data = tfConfVars

	tfConfigPath := path.Join(outDirPath, "tf-config.yaml")
	tfConfig, err := yaml.MarshalV2(config)
	if err != nil {
		return err
	}

	err = os.WriteFile(tfConfigPath, tfConfig, 0o644)
	if err != nil {
		return err
	}

	templateModel, err := template.NewTemplateModel(
		sourceTfDir,
		targetTfDir,
		tfConfigPath,
		outDirPath,
		".tpl",
		true,
		false,
	)
	if err != nil {
		return err
	}

	err = templateModel.Generate()
	return nil
}

func (i *Distribution) CreateFolderStructure() error {
	err := os.Mkdir(i.PlanPath, 0o755)
	if err != nil {
		return err
	}

	err = os.Mkdir(i.LogsPath, 0o755)
	if err != nil {
		return err
	}

	return os.Mkdir(i.OutputsPath, 0o755)
}

func (i *Distribution) TerraformInit() error {
	terraformInitCmd := exec.Command(i.TerraformPath, "init")
	terraformInitCmd.Stdout = os.Stdout
	terraformInitCmd.Stderr = os.Stderr
	terraformInitCmd.Dir = path.Join(i.Path, "terraform")

	return terraformInitCmd.Run()
}

func (i *Distribution) TerraformPlan(timestamp int64) error {
	var planBuffer bytes.Buffer

	terraformPlanCmd := exec.Command(i.TerraformPath, "plan", "--out=plan/terraform.plan", "-no-color")
	terraformPlanCmd.Stdout = io.MultiWriter(os.Stdout, &planBuffer)
	terraformPlanCmd.Stderr = os.Stderr
	terraformPlanCmd.Dir = path.Join(i.Path, "terraform")

	err := terraformPlanCmd.Run()
	if err != nil {
		return err
	}

	logFilePath := fmt.Sprintf("plan-%d.log", timestamp)

	return os.WriteFile(path.Join(i.PlanPath, logFilePath), planBuffer.Bytes(), 0o600)
}

func (i *Distribution) TerraformApply(timestamp int64) (OutputJson, error) {
	var applyBuffer bytes.Buffer
	var applyLogOut OutputJson

	terraformApplyCmd := exec.Command(i.TerraformPath, "apply", "-no-color", "-json", "plan/terraform.plan")
	terraformApplyCmd.Stdout = io.MultiWriter(os.Stdout, &applyBuffer)
	terraformApplyCmd.Stderr = os.Stderr
	terraformApplyCmd.Dir = path.Join(i.Path, "terraform")

	err := terraformApplyCmd.Run()
	if err != nil {
		return applyLogOut, err
	}

	err = os.WriteFile(path.Join(i.LogsPath, fmt.Sprintf("%d.log", timestamp)), applyBuffer.Bytes(), 0o600)
	if err != nil {
		return applyLogOut, err
	}

	parsedApplyLog, err := os.ReadFile(path.Join(i.LogsPath, fmt.Sprintf("%d.log", timestamp)))
	if err != nil {
		return applyLogOut, err
	}

	applyLog := string(parsedApplyLog)

	pattern := regexp.MustCompile("(\"outputs\":){(.*?)}}")

	outputsStringIndex := pattern.FindStringIndex(applyLog)
	if outputsStringIndex == nil {
		return applyLogOut, fmt.Errorf("can't get outputs from terraform apply logs")
	}

	outputsString := fmt.Sprintf("{%s}", applyLog[outputsStringIndex[0]:outputsStringIndex[1]])

	err = json.Unmarshal([]byte(outputsString), &applyLogOut)
	if err != nil {
		return applyLogOut, err
	}

	err = os.WriteFile(path.Join(i.OutputsPath, "output.json"), []byte(outputsString), 0o600)

	return applyLogOut, err
}
