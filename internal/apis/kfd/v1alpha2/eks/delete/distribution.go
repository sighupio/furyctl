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
	"regexp"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/tool/kubectl"
	"github.com/sighupio/furyctl/internal/tool/kustomize"
	"github.com/sighupio/furyctl/internal/tool/terraform"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
)

const (
	ingressAfterDeleteDelay         = 2
	checkPendingResourcesDelay      = 20
	checkPendingResourcesMaxRetries = 5
)

var (
	errCheckPendingResources = errors.New("error while checking pending resources")
	errPendingResources      = errors.New("pending resources: ")
)

type Distribution struct {
	*cluster.OperationPhase
	tfRunner   *terraform.Runner
	kzRunner   *kustomize.Runner
	kubeRunner *kubectl.Runner
	dryRun     bool
}

func NewDistribution(dryRun bool) (*Distribution, error) {
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
		dryRun: dryRun,
	}, nil
}

func (d *Distribution) Exec() error {
	logrus.Info("Deleting distribution phase")

	err := iox.CheckDirIsEmpty(d.OperationPhase.Path)
	if err == nil {
		logrus.Infof("distribution phase already executed, skipping")

		return nil
	}

	logrus.Info("Deleting ingresses")

	if err = d.deleteIngresses(); err != nil {
		return err
	}

	logrus.Info("Building manifests")

	manifestsOutPath, err := d.buildManifests()
	if err != nil {
		return err
	}

	logrus.Info("Deleting manifests")

	err = d.kubeRunner.Delete(manifestsOutPath)
	if err != nil {
		logrus.Errorf("error while deleting resources: %v", err)
	}

	logrus.Info("Checking pending resources")

	err = d.checkPendingResource()
	if err != nil {
		return err
	}

	logrus.Info("Deleting terraform resources")

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

func (d *Distribution) checkPendingResource() error {
	var errSvc, errPv, errIgrs error

	dur := time.Second * checkPendingResourcesDelay

	maxRetries := checkPendingResourcesMaxRetries

	retries := 0

	for retries < maxRetries {
		p := time.NewTicker(dur)

		if <-p.C; true {
			errSvc = d.getLoadBalancers()

			errPv = d.getPersistentVolumes()

			errIgrs = d.getIngresses()

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
	dur := time.Minute * ingressAfterDeleteDelay

	_, err := d.kubeRunner.DeleteAllResources("ingress", "all")
	if err != nil {
		return fmt.Errorf("error deleting ingresses: %w", err)
	}

	time.Sleep(dur)

	return nil
}

func (d *Distribution) getLoadBalancers() error {
	log, err := d.kubeRunner.Get("all", "svc", "-o",
		"jsonpath='{.items[?(@.spec.type==\"LoadBalancer\")].metadata.name}'")
	if err != nil {
		return fmt.Errorf("error while reading resources from cluster: %w", err)
	}

	reg := regexp.MustCompile(`'(.*?)'`)

	logStringIndex := reg.FindStringIndex(log)

	if len(logStringIndex) == 0 {
		return fmt.Errorf("%w: error while parsing kubectl get response", errPendingResources)
	}

	logString := log[logStringIndex[0]:logStringIndex[1]]

	if logString != "''" {
		return fmt.Errorf("%w: %s", errPendingResources, logString)
	}

	return nil
}

func (d *Distribution) getIngresses() error {
	log, err := d.kubeRunner.Get("all", "ingress", "-o", "jsonpath='{.items[*].metadata.name}'")
	if err != nil {
		return fmt.Errorf("error while reading resources from cluster: %w", err)
	}

	reg := regexp.MustCompile(`'(.*?)'`)

	logStringIndex := reg.FindStringIndex(log)

	if len(logStringIndex) == 0 {
		return fmt.Errorf("%w: error while parsing kubectl get response", errPendingResources)
	}

	logString := log[logStringIndex[0]:logStringIndex[1]]

	if logString != "''" {
		return fmt.Errorf("%w: %s", errPendingResources, logString)
	}

	return nil
}

func (d *Distribution) getPersistentVolumes() error {
	log, err := d.kubeRunner.Get("all", "pv", "-o", "jsonpath='{.items[*].metadata.name}'")
	if err != nil {
		return fmt.Errorf("error while reading resources from cluster: %w", err)
	}

	reg := regexp.MustCompile(`'(.*?)'`)

	logStringIndex := reg.FindStringIndex(log)

	if len(logStringIndex) == 0 {
		return fmt.Errorf("%w: error while parsing kubectl get response", errPendingResources)
	}

	logString := log[logStringIndex[0]:logStringIndex[1]]

	if logString != "''" {
		return fmt.Errorf("%w: %s", errPendingResources, logString)
	}

	return nil
}
