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
	OperationPhasePlugins        = "plugins"
	OperationPhaseAll            = ""

	OperationPhaseOptionVPNAutoConnect = "vpnautoconnect"
)

var errUnsupportedPhase = errors.New(
	"unsupported phase, options are: infrastructure, kubernetes, distribution, plugins",
)

func CheckPhase(phase string) error {
	switch phase {
	case OperationPhaseInfrastructure:
	case OperationPhaseKubernetes:
	case OperationPhaseDistribution:
	case OperationPhasePlugins:
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
	Path                 string
	TerraformPath        string
	KustomizePath        string
	KubectlPath          string
	YqPath               string
	HelmPath             string
	HelmfilePath         string
	TerraformPlanPath    string
	TerraformLogsPath    string
	TerraformOutputsPath string
	TerraformSecretsPath string
	binPath              string
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
	yqPath := path.Join(binPath, "yq", kfdTools.Common.Yq.Version, "yq")
	helmPath := path.Join(binPath, "helm", kfdTools.Common.Helm.Version, "helm")
	helmfilePath := path.Join(binPath, "helmfile", kfdTools.Common.Helmfile.Version, "helmfile")

	planPath := path.Join(basePath, "terraform", "plan")
	logsPath := path.Join(basePath, "terraform", "logs")
	outputsPath := path.Join(basePath, "terraform", "outputs")
	secretsPath := path.Join(basePath, "terraform", "secrets")

	return &OperationPhase{
		Path:                 basePath,
		TerraformPath:        terraformPath,
		KustomizePath:        kustomizePath,
		KubectlPath:          kubectlPath,
		TerraformPlanPath:    planPath,
		TerraformLogsPath:    logsPath,
		TerraformOutputsPath: outputsPath,
		TerraformSecretsPath: secretsPath,
		binPath:              binPath,
		YqPath:               yqPath,
		HelmPath:             helmPath,
		HelmfilePath:         helmfilePath,
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
	if _, err := os.Stat(cp.TerraformPlanPath); os.IsNotExist(err) {
		if err := os.Mkdir(cp.TerraformPlanPath, iox.FullPermAccess); err != nil {
			return fmt.Errorf("error creating folder %s: %w", cp.TerraformPlanPath, err)
		}
	}

	if _, err := os.Stat(cp.TerraformLogsPath); os.IsNotExist(err) {
		if err := os.Mkdir(cp.TerraformLogsPath, iox.FullPermAccess); err != nil {
			return fmt.Errorf("error creating folder %s: %w", cp.TerraformLogsPath, err)
		}
	}

	if _, err := os.Stat(cp.TerraformSecretsPath); os.IsNotExist(err) {
		if err := os.Mkdir(cp.TerraformSecretsPath, iox.FullPermAccess); err != nil {
			return fmt.Errorf("error creating folder %s: %w", cp.TerraformSecretsPath, err)
		}
	}

	if _, err := os.Stat(cp.TerraformOutputsPath); os.IsNotExist(err) {
		if err := os.Mkdir(cp.TerraformOutputsPath, iox.FullPermAccess); err != nil {
			return fmt.Errorf("error creating folder %s: %w", cp.TerraformOutputsPath, err)
		}
	}

	return nil
}

func (*OperationPhase) CopyFromTemplate(
	cfg template.Config,
	prefix,
	sourcePath,
	targetPath,
	furyctlConfPath string,
) error {
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
		furyctlConfPath,
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
