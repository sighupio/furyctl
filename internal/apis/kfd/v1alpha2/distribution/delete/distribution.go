// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package del

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/kubernetes"
	"github.com/sighupio/furyctl/internal/tool/kustomize"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
)

const (
	ingressAfterDeleteDelay         = 4
	checkPendingResourcesDelay      = 20
	checkPendingResourcesMaxRetries = 5
)

var (
	errCheckPendingResources = errors.New("error while checking pending resources")
	errPendingResources      = errors.New("pending resources: ")
	errClusterConnect        = errors.New("error connecting to cluster")
)

type Ingress struct {
	Name string
	Host []string
}

type Distribution struct {
	*cluster.OperationPhase
	kzRunner   *kustomize.Runner
	kubeClient *kubernetes.Client
	dryRun     bool
}

func NewDistribution(
	dryRun bool,
	workDir,
	binPath string,
	kfdManifest config.KFD,
	kubeconfig string,
) (*Distribution, error) {
	distroDir := path.Join(workDir, cluster.OperationPhaseDistribution)

	phase, err := cluster.NewOperationPhase(distroDir, kfdManifest.Tools, binPath)
	if err != nil {
		return nil, fmt.Errorf("error creating distribution phase: %w", err)
	}

	return &Distribution{
		OperationPhase: phase,
		kzRunner: kustomize.NewRunner(
			execx.NewStdExecutor(),
			kustomize.Paths{
				Kustomize: phase.KustomizePath,
				WorkDir:   path.Join(phase.Path, "manifests"),
			},
		),
		kubeClient: kubernetes.NewClient(
			phase.KubectlPath,
			path.Join(phase.Path, "manifests"),
			kubeconfig,
			true,
			true,
			false,
			execx.NewStdExecutor(),
		),
		dryRun: dryRun,
	}, nil
}

func (d *Distribution) Exec() error {
	logrus.Info("Deleting Kubernetes Fury Distribution...")

	if err := iox.CheckDirIsEmpty(d.OperationPhase.Path); err == nil {
		logrus.Info("Kubernetes Fury Distribution already deleted, skipping...")

		logrus.Debug("Distribution phase already executed, skipping...")

		return nil
	}

	logrus.Info("Checking cluster connectivity...")

	if _, err := d.kubeClient.ToolVersion(); err != nil {
		return errClusterConnect
	}

	if d.dryRun {
		manifestsOutPath, err := d.buildManifests()
		if err != nil {
			return err
		}

		if _, err = d.kubeClient.DeleteFromPath(manifestsOutPath, "--dry-run=client"); err != nil {
			logrus.Errorf("error while deleting resources: %v", err)
		}

		logrus.Info("The following resources, regardless of the built manifests, are going to be deleted:")

		if _, err := d.kubeClient.ListNamespaceResources("ingress", "all"); err != nil {
			logrus.Errorf("error while getting list of ingress resources: %v", err)
		}

		if _, err := d.kubeClient.ListNamespaceResources("prometheus", "monitoring"); err != nil {
			logrus.Errorf("error while getting list of prometheus resources: %v", err)
		}

		if _, err := d.kubeClient.ListNamespaceResources("persistentvolumeclaim", "monitoring"); err != nil {
			logrus.Errorf("error while getting list of persistentvolumeclaim resources: %v", err)
		}

		if _, err := d.kubeClient.ListNamespaceResources("persistentvolumeclaim", "logging"); err != nil {
			logrus.Errorf("error while getting list of persistentvolumeclaim resources: %v", err)
		}

		if _, err := d.kubeClient.ListNamespaceResources("statefulset", "logging"); err != nil {
			logrus.Errorf("error while getting list of statefulset resources: %v", err)
		}

		if _, err := d.kubeClient.ListNamespaceResources("logging", "logging"); err != nil {
			logrus.Errorf("error while getting list of logging resources: %v", err)
		}

		if _, err := d.kubeClient.ListNamespaceResources("service", "ingress-nginx"); err != nil {
			logrus.Errorf("error while getting list of service resources: %v", err)
		}

		return nil
	}

	logrus.Info("Deleting ingresses...")

	if err := d.deleteIngresses(); err != nil {
		return err
	}

	logrus.Warn("Deleting blocking resources, this operation will take a few minutes!")

	if err := d.deleteBlockingResources(); err != nil {
		return err
	}

	logrus.Info("Building manifests...")

	manifestsOutPath, err := d.buildManifests()
	if err != nil {
		return err
	}

	logrus.Info("Deleting kubernetes resources...")

	_, err = d.kubeClient.DeleteFromPath(manifestsOutPath)
	if err != nil {
		logrus.Errorf("error while deleting resources: %v", err)
	}

	logrus.Info("Checking pending resources...")

	return d.checkPendingResources()
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

func (d *Distribution) checkPendingResources() error {
	var errSvc, errPv, errIgrs error

	var ingrs []kubernetes.Ingress

	var lbs, pvs []string

	dur := time.Second * checkPendingResourcesDelay

	maxRetries := checkPendingResourcesMaxRetries

	retries := 0

	for retries < maxRetries {
		p := time.NewTicker(dur)

		if <-p.C; true {
			lbs, errSvc = d.kubeClient.GetLoadBalancers()
			if errSvc == nil && len(lbs) > 0 {
				errSvc = fmt.Errorf("%w: %v", errPendingResources, lbs)
			}

			pvs, errPv = d.kubeClient.GetPersistentVolumes()
			if errPv == nil && len(pvs) > 0 {
				errPv = fmt.Errorf("%w: %v", errPendingResources, pvs)
			}

			ingrs, errIgrs = d.kubeClient.GetIngresses()
			if errIgrs == nil && len(ingrs) > 0 {
				errIgrs = fmt.Errorf("%w: %v", errPendingResources, ingrs)
			}

			if errSvc == nil && errPv == nil && errIgrs == nil {
				return nil
			}
		}

		retries++

		p.Stop()
	}

	return fmt.Errorf("%w:\n%v\n%v\n%v", errCheckPendingResources, errSvc, errPv, errIgrs)
}

func (d *Distribution) deleteIngresses() error {
	_, err := d.kubeClient.DeleteResourcesInAllNamespaces("ingress")
	if err != nil {
		return fmt.Errorf("error deleting ingresses: %w", err)
	}

	return nil
}

func (d *Distribution) deleteBlockingResources() error {
	if err := d.deleteResource("deployment", "logging", "loki-distributed-distributor"); err != nil {
		return err
	}

	if err := d.deleteResource("deployment", "logging", "loki-distributed-compactor"); err != nil {
		return err
	}

	if err := d.deleteResources("prometheuses.monitoring.coreos.com", "monitoring"); err != nil {
		return err
	}

	if err := d.deleteResources("prometheusrules.monitoring.coreos.com", "monitoring"); err != nil {
		return err
	}

	if err := d.deleteResources("persistentvolumeclaims", "monitoring"); err != nil {
		return err
	}

	if err := d.deleteResources("loggings.logging.banzaicloud.io", "logging"); err != nil {
		return err
	}

	if err := d.deleteResources("statefulsets.apps", "logging"); err != nil {
		return err
	}

	if err := d.deleteResources("persistentvolumeclaims", "logging"); err != nil {
		return err
	}

	if err := d.deleteResources("services", "ingress-nginx"); err != nil {
		return err
	}

	logrus.Debugf("waiting for resources to be deleted...")

	time.Sleep(time.Minute * ingressAfterDeleteDelay)

	return nil
}

func (d *Distribution) deleteResource(typ, ns, name string) error {
	logrus.Infof("Deleting %ss '%s' in namespace '%s'...\n", typ, name, ns)

	resExists, err := d.kubeClient.ResourceExists(name, typ, ns)
	if err != nil {
		return fmt.Errorf("error checking if %s '%s' exists in '%s' namespace: %w", typ, name, ns, err)
	}

	if resExists {
		_, err = d.kubeClient.DeleteResource(name, typ, ns)
		if err != nil {
			return fmt.Errorf("error deleting %s '%s' in '%s' namespace: %w", typ, name, ns, err)
		}
	}

	return nil
}

func (d *Distribution) deleteResources(typ, ns string) error {
	logrus.Infof("Deleting %ss in namespace '%s'...\n", typ, ns)

	hasResTyp, err := d.kubeClient.HasResourceType(typ)
	if err != nil {
		return fmt.Errorf("error checking '%s' resources type: %w", typ, err)
	}

	if !hasResTyp {
		return nil
	}

	_, err = d.kubeClient.DeleteResources(typ, ns)
	if err != nil {
		return fmt.Errorf("error deleting '%s' in namespace '%s': %w", typ, ns, err)
	}

	return nil
}
