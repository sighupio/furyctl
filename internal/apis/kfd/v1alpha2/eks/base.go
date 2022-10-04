// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eks

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"github.com/sighupio/furyctl/internal/iox"
	"github.com/sighupio/furyctl/internal/template"
	"github.com/sighupio/furyctl/internal/yaml"
)

type base struct {
	Path          string
	TerraformPath string
	KustomizePath string
	PlanPath      string
	LogsPath      string
	OutputsPath   string
	SecretsPath   string
	VendorPath    string
}

func newBase(folder string) (*base, error) {
	basePath := path.Join(folder)

	vendorPath, err := filepath.Abs("./vendor")
	if err != nil {
		return &base{}, err
	}

	kustomizePath := path.Join(vendorPath, "bin", "kustomize")
	terraformPath := path.Join(vendorPath, "bin", "terraform")

	planPath := path.Join(basePath, "terraform", "plan")
	logsPath := path.Join(basePath, "terraform", "logs")
	outputsPath := path.Join(basePath, "terraform", "outputs")
	secretsPath := path.Join(basePath, "terraform", "secrets")

	return &base{
		Path:          basePath,
		TerraformPath: terraformPath,
		KustomizePath: kustomizePath,
		PlanPath:      planPath,
		LogsPath:      logsPath,
		OutputsPath:   outputsPath,
		SecretsPath:   secretsPath,
		VendorPath:    vendorPath,
	}, nil
}

func (b *base) createFolder() error {
	return os.Mkdir(b.Path, 0o755)
}

func (b *base) createFolderStructure() error {
	if err := os.Mkdir(b.PlanPath, 0o755); err != nil {
		return err
	}

	if err := os.Mkdir(b.LogsPath, 0o755); err != nil {
		return err
	}

	if err := os.Mkdir(b.SecretsPath, 0o755); err != nil {
		return err
	}

	return os.Mkdir(b.OutputsPath, 0o755)
}

func (b *base) copyFromTemplate(config template.Config, prefix, sourcePath, targetPath string) error {
	outDirPath, err := os.MkdirTemp("", fmt.Sprintf("furyctl-%s-", prefix))
	if err != nil {
		return err
	}

	tfConfigPath := path.Join(outDirPath, "tf-config.yaml")

	tfConfig, err := yaml.MarshalV2(config)
	if err != nil {
		return err
	}

	if err = os.WriteFile(tfConfigPath, tfConfig, 0o644); err != nil {
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

func copyFromFsToDir(src fs.FS, dest string) error {
	stuff, err := fs.ReadDir(src, ".")
	if err != nil {
		return err
	}

	for _, file := range stuff {
		if file.IsDir() {
			sub, err := fs.Sub(src, file.Name())
			if err != nil {
				return err
			}

			if err := copyFromFsToDir(sub, path.Join(dest, file.Name())); err != nil {
				return err
			}

			continue
		}

		fileContent, err := fs.ReadFile(src, file.Name())
		if err != nil {
			return err
		}

		if err := iox.EnsureDir(path.Join(dest, file.Name())); err != nil {
			return err
		}

		if err := os.WriteFile(path.Join(dest, file.Name()), fileContent, 0o600); err != nil {
			return err
		}
	}

	return nil
}
