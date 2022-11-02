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
	OperationPhaseInfrastructure = "infrastructure"
	OperationPhaseKubernetes     = "kubernetes"
	OperationPhaseDistribution   = "distribution"
	OperationPhaseAll            = ""

	OperationPhaseOptionVPNAutoConnect = "vpnautoconnect"
)

type OperationPhase struct {
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

type OperationPhaseOption struct {
	Name  string
	Value any
}

func NewOperationPhase(folder string) (*OperationPhase, error) {
	basePath := path.Join(folder)

	vendorPath, err := filepath.Abs("./vendor")
	if err != nil {
		return &OperationPhase{}, fmt.Errorf("error getting absolute path for vendor folder: %w", err)
	}

	kustomizePath := path.Join(vendorPath, "bin", "kustomize")
	terraformPath := path.Join(vendorPath, "bin", "terraform")
	kubectlPath := path.Join(vendorPath, "bin", "kubectl")

	planPath := path.Join(basePath, "terraform", "plan")
	logsPath := path.Join(basePath, "terraform", "logs")
	outputsPath := path.Join(basePath, "terraform", "outputs")
	secretsPath := path.Join(basePath, "terraform", "secrets")

	return &OperationPhase{
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

func (cp *OperationPhase) CreateFolder() error {
	err := os.Mkdir(cp.Path, iox.FullPermAccess)
	if err != nil {
		return fmt.Errorf("error creating folder %s: %w", cp.Path, err)
	}

	return nil
}

func (cp *OperationPhase) CreateFolderStructure() error {
	if err := os.Mkdir(cp.PlanPath, iox.FullPermAccess); err != nil {
		return fmt.Errorf("error creating folder %s: %w", cp.PlanPath, err)
	}

	if err := os.Mkdir(cp.LogsPath, iox.FullPermAccess); err != nil {
		return fmt.Errorf("error creating folder %s: %w", cp.LogsPath, err)
	}

	if err := os.Mkdir(cp.SecretsPath, iox.FullPermAccess); err != nil {
		return fmt.Errorf("error creating folder %s: %w", cp.SecretsPath, err)
	}

	if err := os.Mkdir(cp.OutputsPath, iox.FullPermAccess); err != nil {
		return fmt.Errorf("error creating folder %s: %w", cp.OutputsPath, err)
	}

	return nil
}

func (*OperationPhase) CopyFromTemplate(config template.Config, prefix, sourcePath, targetPath string) error {
	outDirPath, err := os.MkdirTemp("", fmt.Sprintf("furyctl-%s-", prefix))
	if err != nil {
		return fmt.Errorf("error creating temp folder: %w", err)
	}

	tfConfigPath := path.Join(outDirPath, "tf-config.yaml")

	tfConfig, err := yamlx.MarshalV2(config)
	if err != nil {
		return fmt.Errorf("error marshaling tf-config: %w", err)
	}

	if err = os.WriteFile(tfConfigPath, tfConfig, iox.RWPermAccess); err != nil {
		return fmt.Errorf("error writing tf-config: %w", err)
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
		return fmt.Errorf("error creating template model: %w", err)
	}

	err = templateModel.Generate()
	if err != nil {
		return fmt.Errorf("error generating from template: %w", err)
	}

	return nil
}
