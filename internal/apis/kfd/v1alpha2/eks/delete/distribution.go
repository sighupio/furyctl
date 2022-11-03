// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package del

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/tool/kubectl"
	"github.com/sighupio/furyctl/internal/tool/kustomize"
	"github.com/sighupio/furyctl/internal/tool/terraform"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
)

type Distribution struct {
	*cluster.OperationPhase
	tfRunner   *terraform.Runner
	kzRunner   *kustomize.Runner
	kubeRunner *kubectl.Runner
}

func NewDistribution() (*Distribution, error) {
	phase, err := cluster.NewOperationPhase(".distribution")
	if err != nil {
		return nil, fmt.Errorf("error creating distribution phase: %w", err)
	}

	return &Distribution{
		OperationPhase: phase,
		tfRunner: terraform.NewRunner(
			execx.NewStdExecutor(),
			terraform.Paths{
				Logs:      phase.LogsPath,
				Outputs:   phase.OutputsPath,
				WorkDir:   path.Join(phase.Path, "terraform"),
				Plan:      phase.PlanPath,
				Terraform: phase.TerraformPath,
			},
		),
		kzRunner: kustomize.NewRunner(
			execx.NewStdExecutor(),
			kustomize.Paths{
				Kustomize: phase.KustomizePath,
				WorkDir:   path.Join(phase.Path, "manifests"),
			},
		),
		kubeRunner: kubectl.NewRunner(
			execx.NewStdExecutor(),
			kubectl.Paths{
				Kubectl: phase.KubectlPath,
				WorkDir: path.Join(phase.Path, "manifests"),
			},
			true,
			true,
		),
	}, nil
}

func (d *Distribution) Exec() error {
	err := iox.CheckDirIsEmpty(d.OperationPhase.Path)
	if err == nil {
		logrus.Infof("distribution phase already executed, skipping")

		return nil
	}

	logrus.Info("Building manifests")

	manifestsOutPath, err := d.buildManifests()
	if err != nil {
		return err
	}

	logrus.Info("Deleting manifests")

	err = d.kubeRunner.Delete(manifestsOutPath)
	if err != nil {
		logrus.Errorf("error deleting manifests: %v", err)
	}

	err = d.tfRunner.Destroy()
	if err != nil {
		return fmt.Errorf("error running terraform destroy: %w", err)
	}

	return nil
}

func (d *Distribution) buildManifests() (string, error) {
	kzOut, err := d.kzRunner.Build()
	if err != nil {
		return "", fmt.Errorf("error building manifests: %w", err)
	}

	outDirPath, err := os.MkdirTemp("", "furyctl-dist-manifests-")
	if err != nil {
		return "", fmt.Errorf("error creating temp dir: %w", err)
	}

	manifestsOutPath := filepath.Join(outDirPath, "out.yaml")

	logrus.Debugf("built manifests = %s", manifestsOutPath)

	if err = os.WriteFile(manifestsOutPath, []byte(kzOut), os.ModePerm); err != nil {
		return "", fmt.Errorf("error writing built manifests: %w", err)
	}

	return manifestsOutPath, nil
}
