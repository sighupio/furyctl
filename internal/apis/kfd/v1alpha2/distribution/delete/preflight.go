// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package del

import (
	"errors"
	"fmt"
	"os"
	"path"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/fury-distribution/pkg/apis/kfddistribution/v1alpha2/public"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/tool/kubectl"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	kubex "github.com/sighupio/furyctl/internal/x/kube"
)

var ErrKubeconfigNotSet = errors.New("KUBECONFIG env variable is not set")

type PreFlight struct {
	*cluster.OperationPhase

	furyctlConf public.KfddistributionKfdV1Alpha2
	kubeRunner  *kubectl.Runner
	kfdManifest config.KFD
}

func NewPreFlight(
	furyctlConf public.KfddistributionKfdV1Alpha2,
	kfdManifest config.KFD,
	paths cluster.DeleterPaths,
) *PreFlight {
	phase := cluster.NewOperationPhase(
		path.Join(paths.WorkDir, cluster.OperationPhasePreFlight),
		kfdManifest.Tools,
		paths.BinPath,
	)

	return &PreFlight{
		OperationPhase: phase,
		furyctlConf:    furyctlConf,
		kubeRunner: kubectl.NewRunner(
			execx.NewStdExecutor(),
			kubectl.Paths{
				Kubectl: phase.KubectlPath,
				WorkDir: phase.Path,
			},
			true,
			true,
			false,
		),
		kfdManifest: kfdManifest,
	}
}

func (p *PreFlight) Exec() error {
	logrus.Info("Running preflight checks")

	if err := p.CreateRootFolder(); err != nil {
		return fmt.Errorf("error creating preflight phase folder: %w", err)
	}

	kubeconfigPath := os.Getenv("KUBECONFIG")

	if distribution.HasFeature(p.kfdManifest, distribution.FeatureKubeconfigInSchema) {
		kubeconfigPath = p.furyctlConf.Spec.Distribution.Kubeconfig
	} else if kubeconfigPath == "" {
		return ErrKubeconfigNotSet
	}

	if err := kubex.SetConfigEnv(kubeconfigPath); err != nil {
		return fmt.Errorf("error setting kubeconfig env: %w", err)
	}

	logrus.Info("Checking that the cluster is reachable...")

	if _, err := p.kubeRunner.Version(); err != nil {
		return fmt.Errorf("cluster is unreachable, make sure you have access to the cluster: %w", err)
	}

	logrus.Info("Preflight checks completed successfully")

	return nil
}
