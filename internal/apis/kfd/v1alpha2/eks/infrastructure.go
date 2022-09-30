// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"regexp"

	"github.com/sighupio/fury-distribution/pkg/config"
	"github.com/sighupio/furyctl/configs"
	"github.com/sighupio/furyctl/internal/template"
)

type Infrastructure struct {
	base          *Base
	FuryagentPath string
}

func NewInfrastructure() (*Infrastructure, error) {
	base, err := NewBase(".infrastructure")
	if err != nil {
		return nil, err
	}

	furyAgentPath := path.Join(base.VendorPath, "bin", "furyagent")

	return &Infrastructure{
		base:          base,
		FuryagentPath: furyAgentPath,
	}, nil
}

func (i *Infrastructure) CreateFolder() error {
	return i.base.CreateFolder()
}

func (i *Infrastructure) CreateFolderStructure() error {
	return i.base.CreateFolderStructure()
}

func (i *Infrastructure) Path() string {
	return i.base.Path
}

func (i *Infrastructure) OutputsPath() string {
	return i.base.OutputsPath
}

func (i *Infrastructure) CopyFromTemplate(kfdManifest config.KFD) error {
	var cfg template.Config

	tmpFolder, err := os.MkdirTemp("", "furyctl-infra-configs-")
	if err != nil {
		return err
	}

	defer os.RemoveAll(tmpFolder)

	subFS, err := fs.Sub(configs.Tpl, path.Join("provisioners", "bootstrap", "aws"))
	if err != nil {
		return err
	}

	err = CopyFromFsToDir(subFS, tmpFolder)
	if err != nil {
		return err
	}

	targetTfDir := path.Join(i.base.Path, "terraform")
	prefix := "infra"

	tfConfVars := map[string]map[any]any{
		"kubernetes": {
			"eks": kfdManifest.Kubernetes.Eks,
		},
	}

	cfg.Data = tfConfVars

	return i.base.CopyFromTemplate(
		cfg,
		prefix,
		tmpFolder,
		targetTfDir,
	)
}

func (i *Infrastructure) TerraformInit() error {
	terraformInitCmd := exec.Command(i.base.TerraformPath, "init")
	terraformInitCmd.Stdout = os.Stdout
	terraformInitCmd.Stderr = os.Stderr
	terraformInitCmd.Dir = path.Join(i.base.Path, "terraform")

	return terraformInitCmd.Run()
}

func (i *Infrastructure) TerraformPlan(timestamp int64) error {
	var planBuffer bytes.Buffer

	terraformPlanCmd := exec.Command(i.base.TerraformPath, "plan", "--out=plan/terraform.plan", "-no-color")
	terraformPlanCmd.Stdout = io.MultiWriter(os.Stdout, &planBuffer)
	terraformPlanCmd.Stderr = os.Stderr
	terraformPlanCmd.Dir = path.Join(i.base.Path, "terraform")

	err := terraformPlanCmd.Run()
	if err != nil {
		return err
	}

	logFilePath := fmt.Sprintf("plan-%d.log", timestamp)

	return os.WriteFile(path.Join(i.base.PlanPath, logFilePath), planBuffer.Bytes(), 0o600)
}

//nolint:dupl // it will be refactored
func (i *Infrastructure) TerraformApply(timestamp int64) (OutputJson, error) {
	var applyBuffer bytes.Buffer
	var applyLogOut OutputJson

	terraformApplyCmd := exec.Command(i.base.TerraformPath, "apply", "-no-color", "-json", "plan/terraform.plan")
	terraformApplyCmd.Stdout = io.MultiWriter(os.Stdout, &applyBuffer)
	terraformApplyCmd.Stderr = os.Stderr
	terraformApplyCmd.Dir = path.Join(i.base.Path, "terraform")

	err := terraformApplyCmd.Run()
	if err != nil {
		return applyLogOut, err
	}

	err = os.WriteFile(path.Join(i.base.LogsPath, fmt.Sprintf("%d.log", timestamp)), applyBuffer.Bytes(), 0o600)
	if err != nil {
		return applyLogOut, err
	}

	parsedApplyLog, err := os.ReadFile(path.Join(i.base.LogsPath, fmt.Sprintf("%d.log", timestamp)))
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

	err = os.WriteFile(path.Join(i.base.OutputsPath, "output.json"), []byte(outputsString), 0o600)

	return applyLogOut, err
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
	furyAgentCmd.Dir = i.base.SecretsPath

	err := furyAgentCmd.Run()
	if err != nil {
		return err
	}

	return os.WriteFile(
		path.Join(
			i.base.SecretsPath,
			fmt.Sprintf("%s.ovpn", clientName)),
		furyAgentBuffer.Bytes(),
		0o600,
	)
}

func (i *Infrastructure) CreateOvpnConnection(clientName string) error {
	openVpnCmd := exec.Command(
		"sudo",
		"openvpn",
		"--config",
		fmt.Sprintf("%s.ovpn", clientName),
		"--daemon",
	)
	openVpnCmd.Stdout = os.Stdout
	openVpnCmd.Stderr = os.Stderr
	openVpnCmd.Dir = i.base.SecretsPath

	return openVpnCmd.Run()
}
