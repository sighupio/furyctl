package create

import (
	"fmt"
	"path"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/fury-distribution/pkg/apis/onpremises/v1alpha2/public"
	"github.com/sighupio/furyctl/internal/cluster"
)

type Distribution struct {
	*cluster.OperationPhase
	furyctlConf public.OnpremisesKfdV1Alpha2
	kfdManifest config.KFD
	paths       cluster.CreatorPaths
	dryRun      bool
}

func (d *Distribution) Exec() error {
	logrus.Info("Installing Kubernetes Fury Distribution...")
	logrus.Debug("Create: running distribution phase...")

	logrus.Debug("TODO!")

	logrus.Info("Kubernetes Fury Distribution installed successfully")

	return nil
}

func NewDistribution(
	furyctlConf public.OnpremisesKfdV1Alpha2,
	kfdManifest config.KFD,
	paths cluster.CreatorPaths,
	dryRun bool,
) (*Distribution, error) {
	kubeDir := path.Join(paths.WorkDir, cluster.OperationPhaseDistribution)

	phase, err := cluster.NewOperationPhase(kubeDir, kfdManifest.Tools, paths.BinPath)
	if err != nil {
		return nil, fmt.Errorf("error creating distribution phase: %w", err)
	}

	return &Distribution{
		OperationPhase: phase,
		furyctlConf:    furyctlConf,
		kfdManifest:    kfdManifest,
		paths:          paths,
		dryRun:         dryRun,
	}, nil
}
