// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"

	tfjson "github.com/hashicorp/terraform-json"

	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/template"
	"github.com/sighupio/furyctl/internal/yaml"
)

type Infrastructure struct {
	Path          string
	TerraformPath string
	FuryagentPath string
	PlanPath      string
	LogsPath      string
	OutputsPath   string
	SecretsPath   string
}

func NewInfrastructure() (*Infrastructure, error) {
	infraPath := path.Join(".infrastructure")

	binPath, err := filepath.Abs("./vendor")
	if err != nil {
		return &Infrastructure{}, err
	}

	terraformPath := path.Join(binPath, "bin", "terraform")
	furyAgentPath := path.Join(binPath, "bin", "furyagent")

	planPath := path.Join(infraPath, "terraform", "plan")
	logsPath := path.Join(infraPath, "terraform", "logs")
	outputsPath := path.Join(infraPath, "terraform", "outputs")
	secretsPath := path.Join(infraPath, "terraform", "secrets")

	return &Infrastructure{
		Path:          infraPath,
		TerraformPath: terraformPath,
		FuryagentPath: furyAgentPath,
		PlanPath:      planPath,
		LogsPath:      logsPath,
		OutputsPath:   outputsPath,
		SecretsPath:   secretsPath,
	}, nil
}

func (i *Infrastructure) CreateFolder() error {
	return os.Mkdir(i.Path, 0o755)
}

func (i *Infrastructure) CopyFromTemplate(kfdManifest distribution.Manifest) error {
	var config template.Config

	sourceTfDir := path.Join("configs", "provisioners", "bootstrap", "aws")
	targetTfDir := path.Join(i.Path, "terraform")

	outDirPath, err := os.MkdirTemp("", "furyctl-infra-")
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

func (i *Infrastructure) CreateFolderStructure() error {
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

func (i *Infrastructure) TerraformInit() error {
	terraformInitCmd := exec.Command(i.TerraformPath, "init")
	terraformInitCmd.Stdout = os.Stdout
	terraformInitCmd.Stderr = os.Stderr
	terraformInitCmd.Dir = path.Join(i.Path, "terraform")

	return terraformInitCmd.Run()
}

func (i *Infrastructure) TerraformPlan(timestamp int64) error {
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

func (i *Infrastructure) TerraformApply(timestamp int64) error {
	var applyBuffer bytes.Buffer

	terraformApplyCmd := exec.Command(i.TerraformPath, "apply", "-no-color", "-json", "plan/terraform.plan")
	terraformApplyCmd.Stdout = io.MultiWriter(os.Stdout, &applyBuffer)
	terraformApplyCmd.Stderr = os.Stderr
	terraformApplyCmd.Dir = path.Join(i.Path, "terraform")

	err := terraformApplyCmd.Run()
	if err != nil {
		return err
	}

	err = os.WriteFile(path.Join(i.LogsPath, fmt.Sprintf("%d.log", timestamp)), applyBuffer.Bytes(), 0o600)
	if err != nil {
		return err
	}

	var applyLogOut struct {
		Outputs map[string]*tfjson.StateOutput `json:"outputs"`
	}

	parsedApplyLog, err := ioutil.ReadFile(path.Join(i.LogsPath, fmt.Sprintf("%d.log", timestamp)))
	if err != nil {
		return err
	}

	applyLog := string(parsedApplyLog)

	pattern := regexp.MustCompile("(\"outputs\":){(.*?)}}")

	outputsStringIndex := pattern.FindStringIndex(applyLog)
	if outputsStringIndex == nil {
		return fmt.Errorf("can't get outputs from terraform apply logs")
	}

	outputsString := fmt.Sprintf("{%s}", applyLog[outputsStringIndex[0]:outputsStringIndex[1]])

	err = json.Unmarshal([]byte(outputsString), &applyLogOut)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(path.Join(i.OutputsPath, "output.json"), []byte(outputsString), 0o600)
}

func (i *Infrastructure) CreateOvpnFile(clientName string) error {
	var furyAgentBuffer bytes.Buffer

	furyAgentCmd := exec.Command(i.FuryagentPath,
		"configure",
		"openvpn-client",
		fmt.Sprintf("--client-name=%s", clientName),
		"--config=furyagent.yml",
	)
	furyAgentCmd.Stdout = io.MultiWriter(os.Stdout, &furyAgentBuffer)
	furyAgentCmd.Stderr = os.Stderr
	furyAgentCmd.Dir = i.SecretsPath

	err := furyAgentCmd.Run()
	if err != nil {
		return err
	}

	return os.WriteFile(
		path.Join(
			i.SecretsPath,
			fmt.Sprintf("%s.ovpn", clientName)),
		furyAgentBuffer.Bytes(),
		0o600,
	)
}

func (i *Infrastructure) CreateOvpnConnection(clientName string) error {
	openVpnCmd := exec.Command(
		"openvpn",
		"--config",
		fmt.Sprintf("%s.ovpn", clientName),
		"--daemon",
	)
	openVpnCmd.Stdout = os.Stdout
	openVpnCmd.Stderr = os.Stderr
	openVpnCmd.Dir = i.SecretsPath

	return openVpnCmd.Run()
}
