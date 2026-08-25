// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package onpremises

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/furyctl/internal/apis/config"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/onpremises/public"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/tool/ansible"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	templatex "github.com/sighupio/furyctl/pkg/template"
)

type Renewer struct {
	*cluster.OperationPhase

	furyctlConf public.OnpremisesKfdV1Alpha2
	kfdManifest config.KFD
	distroPath  string
	configPath  string
	binPath     string
}

func (k *Renewer) SetProperties(props []cluster.RenewerProperty) {
	for _, prop := range props {
		k.SetProperty(prop.Name, prop.Value)
	}

	k.OperationPhase = &cluster.OperationPhase{}
}

func (k *Renewer) SetProperty(name string, value any) {
	switch strings.ToLower(name) {
	case cluster.RenewerPropertyFuryctlConf:
		cluster.SetPropertyValue(value, &k.furyctlConf)
	case cluster.RenewerPropertyConfigPath:
		cluster.SetPropertyValue(value, &k.configPath)
	case cluster.RenewerPropertyKfdManifest:
		cluster.SetPropertyValue(value, &k.kfdManifest)
	case cluster.RenewerPropertyDistroPath:
		cluster.SetPropertyValue(value, &k.distroPath)
	case cluster.RenewerPropertyBinPath:
		cluster.SetPropertyValue(value, &k.binPath)
	default:
		logrus.Debugf("ignoring unknown property %q", name)
	}
}

func (k *Renewer) Users() []string {
	names := []string{cluster.AdminKubeconfigUser}

	if k.furyctlConf.Spec.Kubernetes.Advanced != nil && k.furyctlConf.Spec.Kubernetes.Advanced.Users != nil {
		names = append(names, k.furyctlConf.Spec.Kubernetes.Advanced.Users.Names...)
	}

	return names
}

func (k *Renewer) RenewCertificates() error {
	logrus.Info("Renewing certificates...")

	tmpDir, err := os.MkdirTemp("", "fury-renewer-*")
	if err != nil {
		return fmt.Errorf("error creating temporary directory: %w", err)
	}

	defer os.RemoveAll(tmpDir)

	runner, err := k.render(tmpDir)
	if err != nil {
		return err
	}

	if err := cluster.RunPlaybook(
		runner, tmpDir, k.kfdManifest.Version, "98.cluster-certificates-renewal.yaml",
	); err != nil {
		return fmt.Errorf("error renewing certificates: %w", err)
	}

	return nil
}

func (k *Renewer) RenewKubeconfigs(users []string) error {
	logrus.Info("Renewing kubeconfig files...")

	tmpDir, err := os.MkdirTemp("", "fury-renewer-*")
	if err != nil {
		return fmt.Errorf("error creating temporary directory: %w", err)
	}

	defer os.RemoveAll(tmpDir)

	runner, err := k.render(tmpDir)
	if err != nil {
		return err
	}

	extraVars, err := cluster.KubeconfigsExtraVars(users)
	if err != nil {
		return fmt.Errorf("error renewing the kubeconfig files: %w", err)
	}

	if err := cluster.RunPlaybook(
		runner, tmpDir, k.kfdManifest.Version, "99.cluster-kubeconfigs-renewal.yaml", "-e", extraVars,
	); err != nil {
		return fmt.Errorf("error renewing the kubeconfig files: %w", err)
	}

	if err := cluster.CopyRenewedKubeconfigs(tmpDir, users); err != nil {
		return fmt.Errorf("error collecting the renewed kubeconfig files: %w", err)
	}

	return nil
}

// render writes the kubernetes phase templates into workDir and returns an ansible runner rooted
// there, with the hosts already checked.
func (k *Renewer) render(workDir string) (*ansible.Runner, error) {
	ansibleRunner := ansible.NewRunner(
		execx.NewStdExecutor(),
		ansible.PathsForVersion(k.binPath, k.kfdManifest.Tools.OnPremises.Ansible.Version, workDir),
	)

	furyctlMerger, err := k.CreateFuryctlMerger(
		k.distroPath,
		k.configPath,
		"kfd-v1alpha2",
		"onpremises",
	)
	if err != nil {
		return nil, fmt.Errorf("error creating furyctl merger: %w", err)
	}

	mCfg, err := templatex.NewConfigWithoutData(furyctlMerger, []string{})
	if err != nil {
		return nil, fmt.Errorf("error creating template config: %w", err)
	}

	mCfg.Data["kubernetes"] = map[any]any{
		"version": k.kfdManifest.Kubernetes.OnPremises.Version,
	}

	cluster.SetEmptyRenderDefaults(&mCfg)

	if err := k.CopyFromTemplate(
		mCfg,
		"kubernetes",
		path.Join(k.distroPath, "templates", cluster.OperationPhaseKubernetes, "onpremises"),
		workDir,
		k.configPath,
	); err != nil {
		return nil, fmt.Errorf("error copying from template: %w", err)
	}

	if _, err := ansibleRunner.Exec("all", "-m", "ping"); err != nil {
		return nil, fmt.Errorf("error checking hosts: %w", err)
	}

	return ansibleRunner, nil
}
