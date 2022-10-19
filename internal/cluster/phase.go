// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cluster

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/sighupio/furyctl/internal/template"
	iox "github.com/sighupio/furyctl/internal/x/io"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

const (
	CreationPhaseInfrastructure = "infrastructure"
	CreationPhaseKubernetes     = "kubernetes"
	CreationPhaseDistribution   = "distribution"
	CreationPhaseAll            = ""

	CreationPhaseOptionVPNAutoConnect = "vpnautoconnect"
)

type CreationPhaseOption struct {
	Name  string
	Value any
}

func NewCreationPhase(folder string) (*CreationPhase, error) {
	basePath := path.Join(folder)

	vendorPath, err := filepath.Abs("./vendor")
	if err != nil {
		return &CreationPhase{}, err
	}

	kustomizePath := path.Join(vendorPath, "bin", "kustomize")
	terraformPath := path.Join(vendorPath, "bin", "terraform")
	kubectlPath := path.Join(vendorPath, "bin", "kubectl")

	planPath := path.Join(basePath, "terraform", "plan")
	logsPath := path.Join(basePath, "terraform", "logs")
	outputsPath := path.Join(basePath, "terraform", "outputs")
	secretsPath := path.Join(basePath, "terraform", "secrets")

	return &CreationPhase{
		Path:          basePath,
		TerraformPath: terraformPath,
		KustomizePath: kustomizePath,
		KubectlPath:   kubectlPath,
		PlanPath:      planPath,
		LogsPath:      logsPath,
		OutputsPath:   outputsPath,
		SecretsPath:   secretsPath,
		VendorPath:    vendorPath,
	}, nil
}

type CreationPhase struct {
	Path          string
	TerraformPath string
	KustomizePath string
	KubectlPath   string
	PlanPath      string
	LogsPath      string
	OutputsPath   string
	SecretsPath   string
	VendorPath    string
}

func (cp *CreationPhase) CreateFolder() error {
	return os.Mkdir(cp.Path, iox.FullPermAccess)
}

func (cp *CreationPhase) CreateFolderStructure() error {
	if err := os.Mkdir(cp.PlanPath, iox.FullPermAccess); err != nil {
		return err
	}

	if err := os.Mkdir(cp.LogsPath, iox.FullPermAccess); err != nil {
		return err
	}

	if err := os.Mkdir(cp.SecretsPath, iox.FullPermAccess); err != nil {
		return err
	}

	return os.Mkdir(cp.OutputsPath, iox.FullPermAccess)
}

func (cp *CreationPhase) CopyFromTemplate(config template.Config, prefix, sourcePath, targetPath string) error {
	outDirPath, err := os.MkdirTemp("", fmt.Sprintf("furyctl-%s-", prefix))
	if err != nil {
		return err
	}

	tfConfigPath := path.Join(outDirPath, "tf-config.yaml")

	tfConfig, err := yamlx.MarshalV2(config)
	if err != nil {
		return err
	}

	if err = os.WriteFile(tfConfigPath, tfConfig, iox.RWPermAccess); err != nil {
		return err
	}

	templateModel, err := template.NewTemplateModel(
		sourcePath,
		targetPath,
		tfConfigPath,
		outDirPath,
		".tpl",
		true,
		false,
	)
	if err != nil {
		return err
	}

	return templateModel.Generate()
}
