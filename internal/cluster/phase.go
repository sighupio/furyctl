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
	OperationPhasePreFlight             = "preflight"
	OperationPhaseInfrastructure        = "infrastructure"
	OperationSubPhasePreInfrastructure  = "pre-infrastructure"
	OperationSubPhasePostInfrastructure = "post-infrastructure"
	OperationPhaseKubernetes            = "kubernetes"
	OperationSubPhasePreKubernetes      = "pre-kubernetes"
	OperationSubPhasePostKubernetes     = "post-kubernetes"
	OperationPhaseDistribution          = "distribution"
	OperationSubPhasePreDistribution    = "pre-distribution"
	OperationSubPhasePostDistribution   = "post-distribution"
	OperationPhasePlugins               = "plugins"
	OperationPhasePreUpgrade            = "pre-upgrade"
	OperationPhaseAll                   = ""

	OperationPhaseOptionVPNAutoConnect = "vpnautoconnect"
)

var (
	ErrUnsupportedPhase = errors.New(
		"unsupported phase, options are: infrastructure, kubernetes, distribution, plugins",
	)
	ErrUnsupportedOperationPhase = errors.New(
		"unsupported operation phase, options are: pre-infrastructure, infrastructure, post-infrastructure, " +
			"pre-kubernetes, kubernetes, post-kubernetes, pre-distribution, distribution, post-distribution, plugins",
	)
)

func CheckPhase(phase string) error {
	switch phase {
	case OperationPhasePreFlight,
		OperationPhaseInfrastructure,
		OperationPhaseKubernetes,
		OperationPhaseDistribution,
		OperationPhasePlugins,
		OperationPhaseAll:
		return nil

	default:
		return ErrUnsupportedPhase
	}
}

func ValidateOperationPhase(phase string) error {
	err := CheckPhase(phase)
	if err == nil {
		return nil
	}

	switch phase {
	case OperationSubPhasePreInfrastructure,
		OperationSubPhasePostInfrastructure,
		OperationSubPhasePreKubernetes,
		OperationSubPhasePostKubernetes,
		OperationSubPhasePreDistribution,
		OperationSubPhasePostDistribution:
		return nil

	default:
		return ErrUnsupportedOperationPhase
	}
}

func GetPhasesOrder() []string {
	return []string{
		"PreInfrastructure",
		"Infrastructure",
		"PostInfrastructure",
		"PreKubernetes",
		"Kubernetes",
		"PostKubernetes",
		"PreDistribution",
		"Distribution",
		"PostDistribution",
	}
}

func GetPhase(phase string) string {
	switch phase {
	case "PreInfrastructure":
		return OperationSubPhasePreInfrastructure

	case "Infrastructure":
		return OperationPhaseInfrastructure

	case "PostInfrastructure":
		return OperationSubPhasePostInfrastructure

	case "PreKubernetes":
		return OperationSubPhasePreKubernetes

	case "Kubernetes":
		return OperationPhaseKubernetes

	case "PostKubernetes":
		return OperationSubPhasePostKubernetes

	case "PreDistribution":
		return OperationSubPhasePreDistribution

	case "Distribution":
		return OperationPhaseDistribution

	case "PostDistribution":
		return OperationSubPhasePostDistribution

	case "":
		return OperationPhaseAll

	default:
		return ""
	}
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

func NewOperationPhase(folder string, kfdTools config.KFDTools, binPath string) *OperationPhase {
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
	}
}

func (op *OperationPhase) CreateFolder() error {
	if _, err := os.Stat(op.Path); !os.IsNotExist(err) {
		return nil
	}

	err := os.Mkdir(op.Path, iox.FullPermAccess)
	if err != nil {
		return fmt.Errorf("error creating folder %s: %w", op.Path, err)
	}

	return nil
}

func (op *OperationPhase) CreateFolderStructure() error {
	if _, err := os.Stat(op.TerraformPlanPath); os.IsNotExist(err) {
		if err := os.Mkdir(op.TerraformPlanPath, iox.FullPermAccess); err != nil {
			return fmt.Errorf("error creating folder %s: %w", op.TerraformPlanPath, err)
		}
	}

	if _, err := os.Stat(op.TerraformLogsPath); os.IsNotExist(err) {
		if err := os.Mkdir(op.TerraformLogsPath, iox.FullPermAccess); err != nil {
			return fmt.Errorf("error creating folder %s: %w", op.TerraformLogsPath, err)
		}
	}

	if _, err := os.Stat(op.TerraformSecretsPath); os.IsNotExist(err) {
		if err := os.Mkdir(op.TerraformSecretsPath, iox.FullPermAccess); err != nil {
			return fmt.Errorf("error creating folder %s: %w", op.TerraformSecretsPath, err)
		}
	}

	if _, err := os.Stat(op.TerraformOutputsPath); os.IsNotExist(err) {
		if err := os.Mkdir(op.TerraformOutputsPath, iox.FullPermAccess); err != nil {
			return fmt.Errorf("error creating folder %s: %w", op.TerraformOutputsPath, err)
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

func (op *OperationPhase) CopyPathsToConfig(cfg *template.Config) {
	cfg.Data["paths"] = map[any]any{
		"helm":       op.HelmPath,
		"helmfile":   op.HelmfilePath,
		"kubectl":    op.KubectlPath,
		"kustomize":  op.KustomizePath,
		"terraform":  op.TerraformPath,
		"vendorPath": path.Join(op.Path, "..", "vendor"),
		"yq":         op.YqPath,
	}
}

func (op *OperationPhase) Self() *OperationPhase {
	return op
}
