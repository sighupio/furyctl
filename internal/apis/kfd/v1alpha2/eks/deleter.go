// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eks

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/config"
	del "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/eks/delete"
	"github.com/sighupio/furyctl/internal/cluster"
)

type ClusterDeleter struct {
	kfdManifest config.KFD
	phase       string
	workDir     string
	binPath     string
	kubeconfig  string
}

func (d *ClusterDeleter) SetProperties(props []cluster.DeleterProperty) {
	for _, prop := range props {
		d.SetProperty(prop.Name, prop.Value)
	}
}

func (d *ClusterDeleter) SetProperty(name string, value any) {
	lcName := strings.ToLower(name)

	switch lcName {
	case cluster.DeleterPropertyKfdManifest:
		if kfdManifest, ok := value.(config.KFD); ok {
			d.kfdManifest = kfdManifest
		}

	case cluster.DeleterPropertyPhase:
		if s, ok := value.(string); ok {
			d.phase = s
		}

	case cluster.DeleterPropertyWorkDir:
		if s, ok := value.(string); ok {
			d.workDir = s
		}

	case cluster.DeleterPropertyBinPath:
		if s, ok := value.(string); ok {
			d.binPath = s
		}

	case cluster.DeleterPropertyKubeconfig:
		if s, ok := value.(string); ok {
			d.kubeconfig = s
		}
	}
}

func (d *ClusterDeleter) Delete(dryRun bool) error {
	distro, err := del.NewDistribution(dryRun, d.workDir, d.binPath, d.kfdManifest, d.kubeconfig)
	if err != nil {
		return fmt.Errorf("error while creating distribution phase: %w", err)
	}

	kube, err := del.NewKubernetes(dryRun, d.workDir, d.binPath, d.kfdManifest)
	if err != nil {
		return fmt.Errorf("error while creating kubernetes phase: %w", err)
	}

	infra, err := del.NewInfrastructure(dryRun, d.workDir, d.binPath, d.kfdManifest)
	if err != nil {
		return fmt.Errorf("error while creating infrastructure phase: %w", err)
	}

	switch d.phase {
	case cluster.OperationPhaseInfrastructure:
		if err := infra.Exec(); err != nil {
			return fmt.Errorf("error while deleting infrastructure phase: %w", err)
		}

		logrus.Info("Infrastructure deleted successfully")

		return nil

	case cluster.OperationPhaseKubernetes:
		if err := kube.Exec(); err != nil {
			return fmt.Errorf("error while deleting kubernetes phase: %w", err)
		}

		logrus.Info("Kubernetes cluster deleted successfully")

		return nil

	case cluster.OperationPhaseDistribution:
		if err := distro.Exec(); err != nil {
			return fmt.Errorf("error while deleting distribution phase: %w", err)
		}

		logrus.Info("Kubernetes Fury Distribution deleted successfully")

		return nil

	case cluster.OperationPhaseAll:
		if err := distro.Exec(); err != nil {
			return fmt.Errorf("error while deleting distribution phase: %w", err)
		}

		if err := kube.Exec(); err != nil {
			return fmt.Errorf("error while deleting kubernetes phase: %w", err)
		}

		if err := infra.Exec(); err != nil {
			return fmt.Errorf("error while deleting infrastructure phase: %w", err)
		}

		logrus.Info("Kubernetes Fury cluster deleted successfully")

		return nil

	default:
		return ErrUnsupportedPhase
	}
}
