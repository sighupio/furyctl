// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cluster

import (
	"errors"
	"fmt"
	"os"
	"path"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
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

var errUnsupportedPhase = errors.New("unsupported phase, options are: infrastructure, kubernetes, distribution")

func CheckPhase(phase string) error {
	switch phase {
	case OperationPhaseInfrastructure:
	case OperationPhaseKubernetes:
	case OperationPhaseDistribution:
	case OperationPhaseAll:
		{
			break
		}

	default:
		return errUnsupportedPhase
	}

	return nil
}

type OperationPhase struct {
	Path          string
	TerraformPath string
	KustomizePath string
	KubectlPath   string
	PlanPath      string
	LogsPath      string
	OutputsPath   string
	SecretsPath   string
	binPath       string
}

type OperationPhaseOption struct {
	Name  string
	Value any
}

func NewOperationPhase(folder string, kfdTools config.KFDTools, binPath string) (*OperationPhase, error) {
	basePath := folder

	kustomizePath := path.Join(binPath, "kustomize", kfdTools.Common.Kustomize.Version, "kustomize")
	terraformPath := path.Join(binPath, "terraform", kfdTools.Common.Terraform.Version, "terraform")
	kubectlPath := path.Join(binPath, "kubectl", kfdTools.Common.Kubectl.Version, "kubectl")

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
		binPath:       binPath,
	}, nil
}

func (cp *OperationPhase) CreateFolder() error {
	if _, err := os.Stat(cp.Path); !os.IsNotExist(err) {
		return nil
	}

	err := os.Mkdir(cp.Path, iox.FullPermAccess)
	if err != nil {
		return fmt.Errorf("error creating folder %s: %w", cp.Path, err)
	}

	return nil
}

func (cp *OperationPhase) CreateFolderStructure() error {
	if _, err := os.Stat(cp.PlanPath); os.IsNotExist(err) {
		if err := os.Mkdir(cp.PlanPath, iox.FullPermAccess); err != nil {
			return fmt.Errorf("error creating folder %s: %w", cp.PlanPath, err)
		}
	}

	if _, err := os.Stat(cp.LogsPath); os.IsNotExist(err) {
		if err := os.Mkdir(cp.LogsPath, iox.FullPermAccess); err != nil {
			return fmt.Errorf("error creating folder %s: %w", cp.LogsPath, err)
		}
	}

	if _, err := os.Stat(cp.SecretsPath); os.IsNotExist(err) {
		if err := os.Mkdir(cp.SecretsPath, iox.FullPermAccess); err != nil {
			return fmt.Errorf("error creating folder %s: %w", cp.SecretsPath, err)
		}
	}

	if _, err := os.Stat(cp.OutputsPath); os.IsNotExist(err) {
		if err := os.Mkdir(cp.OutputsPath, iox.FullPermAccess); err != nil {
			return fmt.Errorf("error creating folder %s: %w", cp.OutputsPath, err)
		}
	}

	return nil
}

func (*OperationPhase) CopyFromTemplate(cfg template.Config, prefix, sourcePath, targetPath string) error {
	outDirPath, err := os.MkdirTemp("", fmt.Sprintf("furyctl-%s-", prefix))
	if err != nil {
		return fmt.Errorf("error creating temp folder: %w", err)
	}

	tfConfigPath := path.Join(outDirPath, "tf-config.yaml")

	tfConfig, err := yamlx.MarshalV2(cfg)
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
		false,
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
