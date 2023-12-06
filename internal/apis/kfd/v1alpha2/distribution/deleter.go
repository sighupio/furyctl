// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package distribution

import (
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/fury-distribution/pkg/apis/kfddistribution/v1alpha2/public"
	del "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/distribution/delete"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/distribution"
	kubex "github.com/sighupio/furyctl/internal/x/kube"
)

type ClusterDeleter struct {
	paths       cluster.DeleterPaths
	kfdManifest config.KFD
	furyctlConf public.KfddistributionKfdV1Alpha2
	phase       string
	dryRun      bool
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

	case cluster.DeleterPropertyFuryctlConf:
		if s, ok := value.(public.KfddistributionKfdV1Alpha2); ok {
			d.furyctlConf = s
		}

	case cluster.DeleterPropertyPhase:
		if s, ok := value.(string); ok {
			d.phase = s
		}

	case cluster.DeleterPropertyWorkDir:
		if s, ok := value.(string); ok {
			d.paths.WorkDir = s
		}

	case cluster.DeleterPropertyBinPath:
		if s, ok := value.(string); ok {
			d.paths.BinPath = s
		}

	case cluster.DeleterPropertyDryRun:
		if b, ok := value.(bool); ok {
			d.dryRun = b
		}
	}
}

func (d *ClusterDeleter) Delete() error {
	if d.phase != "" && d.phase != cluster.OperationPhaseDistribution {
		return ErrUnsupportedPhase
	}

	distro := del.NewDistribution(d.furyctlConf, d.dryRun, d.paths.WorkDir, d.paths.BinPath, d.kfdManifest)

	kubeconfigPath := os.Getenv("KUBECONFIG")

	if distribution.HasFeature(d.kfdManifest, distribution.FeatureKubeconfigInSchema) {
		kubeconfigPath = d.furyctlConf.Spec.Distribution.Kubeconfig
	}

	// Move this code to delete preflight.
	if err := kubex.SetConfigEnv(kubeconfigPath); err != nil {
		return fmt.Errorf("error setting kubeconfig env: %w", err)
	}

	if err := distro.Exec(); err != nil {
		return fmt.Errorf("error while deleting distribution: %w", err)
	}

	logrus.Info("Kubernetes Fury Distribution deleted successfully")

	return nil
}
