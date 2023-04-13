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
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/config"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/kubernetes"
	"github.com/sighupio/furyctl/internal/tool/awscli"
	"github.com/sighupio/furyctl/internal/tool/kustomize"
	"github.com/sighupio/furyctl/internal/tool/terraform"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
	"github.com/sighupio/furyctl/internal/x/slices"
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
	hostedZoneRegex          = regexp.MustCompile(`/hostedzone/(\S+)\t(\S+)\.`)
	recordSetsRegex          = regexp.MustCompile(`(\S+)\.`)
)

type Ingress struct {
	Name string
	Host []string
}

type Distribution struct {
	*cluster.OperationPhase
	tfRunner   *terraform.Runner
	kzRunner   *kustomize.Runner
	awsRunner  *awscli.Runner
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
		awsRunner: awscli.NewRunner(
			execx.NewStdExecutor(),
			awscli.Paths{
				Awscli:  "aws",
				WorkDir: phase.Path,
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

	logrus.Debug("Delete: running distribution phase...")

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
		timestamp := time.Now().Unix()

		if err := d.tfRunner.Init(); err != nil {
			return fmt.Errorf("error running terraform init: %w", err)
		}

		if err := d.tfRunner.Plan(timestamp, "-destroy"); err != nil {
			return fmt.Errorf("error running terraform plan: %w", err)
		}

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

	ingressHosts, err := d.getIngressHosts()
	if err != nil {
		return err
	}

	hostedZones, err := d.getHostedZones()
	if err != nil {
		return err
	}

	logrus.Info("Deleting ingresses...")

	if err = d.deleteIngresses(); err != nil {
		return err
	}

	if len(ingressHosts) > 0 {
		logrus.Info("Waiting for DNS records to be deleted...")

		if err = d.assertEmptyDNSRecords(ingressHosts, hostedZones); err != nil {
			return err
		}
	}

	logrus.Warn("Deleting blocking resources, this operation will take a few minutes!")

	if err = d.deleteBlockingResources(); err != nil {
		return err
	}

	logrus.Info("Building manifests...")

	manifestsOutPath, err := d.buildManifests()
	if err != nil {
		return err
	}

	logrus.Info("Deleting manifests...")

	_, err = d.kubeClient.DeleteFromPath(manifestsOutPath)
	if err != nil {
		logrus.Errorf("error while deleting resources: %v", err)
	}

	logrus.Info("Checking pending resources...")

	if err = d.checkPendingResources(); err != nil {
		return err
	}

	logrus.Info("Deleting infra resources...")

	if err = d.tfRunner.Destroy(); err != nil {
		return fmt.Errorf("error while deleting infra resources: %w", err)
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
	_, err := d.kubeClient.DeleteAllResources("ingress", "all")
	if err != nil {
		return fmt.Errorf("error deleting ingresses: %w", err)
	}

	return nil
}

func (d *Distribution) deleteBlockingResources() error {
	dur := time.Minute * ingressAfterDeleteDelay

	logrus.Info("Deleting prometheus resources...")

	_, err := d.kubeClient.DeleteAllResources("prometheus", "monitoring")
	if err != nil {
		return fmt.Errorf("error deleting prometheus resources: %w", err)
	}

	logrus.Info("Deleting PersistentVolumeClaims in the namespace 'monitoring'...")

	_, err = d.kubeClient.DeleteAllResources("pvc", "monitoring")
	if err != nil {
		return fmt.Errorf("error deleting pvc in namespace 'monitoring': %w", err)
	}

	logrus.Info("Deleting logging resources...")

	_, err = d.kubeClient.DeleteAllResources("logging", "logging")
	if err != nil {
		return fmt.Errorf("error deleting logging resources: %w", err)
	}

	logrus.Info("Deleting StafultSets in the namespace 'logging'...")

	_, err = d.kubeClient.DeleteAllResources("sts", "logging")
	if err != nil {
		return fmt.Errorf("error deleting sts in namespace 'logging': %w", err)
	}

	logrus.Info("Deleting PersistentVolumeClaims in the namespace 'logging'...")

	_, err = d.kubeClient.DeleteAllResources("pvc", "logging")
	if err != nil {
		return fmt.Errorf("error deleting pvc in namespace 'logging': %w", err)
	}

	logrus.Info("Deleting Services in the namespace 'ingress-nginx'...")

	_, err = d.kubeClient.DeleteAllResources("svc", "ingress-nginx")
	if err != nil {
		return fmt.Errorf("error deleting svc in namespace 'ingress-nginx': %w", err)
	}

	logrus.Debugf("waiting for resources to be deleted...")
	time.Sleep(dur)

	return nil
}

func (d *Distribution) getIngressHosts() ([]string, error) {
	ingrs, err := d.kubeClient.GetIngresses()
	if err != nil {
		return nil, fmt.Errorf("error getting ingresses: %w", err)
	}

	var hosts []string

	for _, ingress := range ingrs {
		hosts = append(hosts, ingress.Host...)
	}

	hosts = slices.Uniq(hosts)

	return hosts, nil
}

func (d *Distribution) getHostedZones() (map[string]string, error) {
	zones := make(map[string]string)

	route53, err := d.awsRunner.Route53("list-hosted-zones", "--query", "HostedZones[*].[Id,Name]",
		"--output", "text")
	if err != nil {
		return zones, fmt.Errorf("error getting hosted zones: %w", err)
	}

	matches := hostedZoneRegex.FindAllStringSubmatch(route53, -1)

	for _, match := range matches {
		zones[match[1]] = match[2]
	}

	return zones, nil
}

func (d *Distribution) assertEmptyDNSRecords(hosts []string, hostedZones map[string]string) error {
	if len(hosts) == 0 {
		return nil
	}

	queue := make([]string, 0, len(hostedZones))

	for zone := range hostedZones {
		queue = append(queue, zone)
	}

	trimSuffix := func(a string) string {
		return strings.TrimSuffix(a, ".")
	}

	dur := time.Second * checkPendingResourcesDelay

	maxRetries := checkPendingResourcesMaxRetries * len(hosts)

	retries := 0

	for retries < maxRetries {
		p := time.NewTicker(dur)

		if <-p.C; true {
			for _, zone := range queue {
				domains, err := d.awsRunner.Route53("list-resource-record-sets", "--hosted-zone-id", zone,
					"--query", "ResourceRecordSets[*].Name", "--output", "text")
				if err != nil {
					return fmt.Errorf("error getting hosted zone records: %w", err)
				}

				matches := recordSetsRegex.FindAllString(domains, -1)

				if slices.DisjointTransform(
					hosts,
					matches,
					nil,
					trimSuffix,
				) {
					queue = queue[1:]
				}
			}

			if len(queue) == 0 {
				return nil
			}
		}

		retries++

		p.Stop()
	}

	return fmt.Errorf("%w: hostedzones %v", errCheckPendingResources, queue)
}
