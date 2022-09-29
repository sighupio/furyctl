// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eks

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/sighupio/furyctl/internal/template"
	"github.com/sighupio/furyctl/internal/yaml"
)

type Base struct {
	Path          string
	TerraformPath string
	PlanPath      string
	LogsPath      string
	OutputsPath   string
	SecretsPath   string
	VendorPath    string
}

func NewBase(folder string) (*Base, error) {
	basePath := path.Join(folder)

	vendorPath, err := filepath.Abs("./vendor")
	if err != nil {
		return &Base{}, err
	}

	terraformPath := path.Join(vendorPath, "bin", "terraform")

	planPath := path.Join(basePath, "terraform", "plan")
	logsPath := path.Join(basePath, "terraform", "logs")
	outputsPath := path.Join(basePath, "terraform", "outputs")
	secretsPath := path.Join(basePath, "terraform", "secrets")

	return &Base{
		Path:          basePath,
		TerraformPath: terraformPath,
		PlanPath:      planPath,
		LogsPath:      logsPath,
		OutputsPath:   outputsPath,
		SecretsPath:   secretsPath,
		VendorPath:    vendorPath,
	}, nil
}

func (b *Base) CreateFolder() error {
	return os.Mkdir(b.Path, 0o755)
}

func (b *Base) CreateFolderStructure() error {
	err := os.Mkdir(b.PlanPath, 0o755)
	if err != nil {
		return err
	}

	err = os.Mkdir(b.LogsPath, 0o755)
	if err != nil {
		return err
	}

	err = os.Mkdir(b.SecretsPath, 0o755)
	if err != nil {
		return err
	}

	return os.Mkdir(b.OutputsPath, 0o755)
}

func (b *Base) CopyFromTemplate(config template.Config, prefix, sourcePath, targetPath string) error {
	outDirPath, err := os.MkdirTemp("", fmt.Sprintf("furyctl-%s-", prefix))
	if err != nil {
		return err
	}

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
