// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package onpremises

import (
	"errors"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/furyctl/internal/apis/config"
	del "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/onpremises/delete"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/onpremises/public"
	"github.com/sighupio/furyctl/internal/cluster"
)

var errClusterNotFound = errors.New("the cluster does not exist")

type ClusterDeleter struct {
	paths       cluster.DeleterPaths
	furyctlConf public.OnpremisesKfdV1Alpha2
	kfdManifest config.KFD
	phase       string
	dryRun      bool
}

func (d *ClusterDeleter) SetProperties(props []cluster.DeleterProperty) {
	for _, prop := range props {
		d.SetProperty(prop.Name, prop.Value)
	}
}

func (d *ClusterDeleter) SetProperty(name string, value any) {
	switch strings.ToLower(name) {
	case cluster.DeleterPropertyDistroPath:
		cluster.SetPropertyValue(value, &d.paths.DistroPath)
	case cluster.DeleterPropertyWorkDir:
		cluster.SetPropertyValue(value, &d.paths.WorkDir)
	case cluster.DeleterPropertyBinPath:
		cluster.SetPropertyValue(value, &d.paths.BinPath)
	case cluster.DeleterPropertyConfigPath:
		cluster.SetPropertyValue(value, &d.paths.ConfigPath)
	case cluster.CreatorPropertyFuryctlConf:
		cluster.SetPropertyValue(value, &d.furyctlConf)
	case cluster.CreatorPropertyKfdManifest:
		cluster.SetPropertyValue(value, &d.kfdManifest)
	case cluster.CreatorPropertyPhase:
		cluster.SetPropertyValue(value, &d.phase)
	case cluster.CreatorPropertyDryRun:
		cluster.SetPropertyValue(value, &d.dryRun)
	default:
		logrus.Debugf("ignoring unknown property %q", name)
	}
}

func (d *ClusterDeleter) Delete() error {
	logrus.Warn("This process will only reset the Kubernetes cluster " +
		"and will not uninstall all the packages installed on the nodes.")

	kubernetesPhase := del.NewKubernetes(
		d.furyctlConf,
		d.kfdManifest,
		d.paths,
		d.dryRun,
	)

	distributionPhase := del.NewDistribution(
		d.furyctlConf,
		d.kfdManifest,
		d.paths,
		d.dryRun,
	)

	preflight := del.NewPreFlight(d.furyctlConf, d.kfdManifest, d.paths, d.dryRun)

	clusterExists, err := preflight.Exec()
	if err != nil {
		return fmt.Errorf("error while executing preflight phase: %w", err)
	}

	deleteDistribution, err := distributionDeletion(d.phase, clusterExists)
	if err != nil {
		return err
	}

	if deleteDistribution {
		if err := distributionPhase.Exec(); err != nil {
			return fmt.Errorf("error while deleting distribution phase: %w", err)
		}
	}

	if d.phase != cluster.OperationPhaseDistribution {
		if err := kubernetesPhase.Exec(); err != nil {
			return fmt.Errorf("error while deleting kubernetes phase: %w", err)
		}
	}

	return nil
}

// distributionDeletion reports whether a delete runs the distribution phase. Without a cluster the
// preflight check sets no KUBECONFIG, and the distribution phase would then delete the manifests
// from the cluster of the current kubeconfig context, so it must not run.
//
//nolint:revive // clusterExists is the result of the preflight check, not a mode.
func distributionDeletion(phase string, clusterExists bool) (bool, error) {
	switch phase {
	case cluster.OperationPhaseKubernetes:
		return false, nil

	case cluster.OperationPhaseDistribution, cluster.OperationPhaseAll:
		if clusterExists {
			return true, nil
		}

		if phase == cluster.OperationPhaseDistribution {
			return false, fmt.Errorf(
				"%w: the preflight check read no kubeconfig from the control plane hosts, so there is no "+
					"distribution to delete",
				errClusterNotFound,
			)
		}

		logrus.Info("The preflight check read no kubeconfig from the control plane hosts, skipping the " +
			"distribution phase...")

		return false, nil

	default:
		return false, ErrUnsupportedPhase
	}
}
