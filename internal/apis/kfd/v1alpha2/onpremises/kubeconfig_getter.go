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
	iox "github.com/sighupio/furyctl/internal/x/io"
	templatex "github.com/sighupio/furyctl/pkg/template"
)

type KubeconfigGetter struct {
	*cluster.OperationPhase

	furyctlConf public.OnpremisesKfdV1Alpha2
	kfdManifest config.KFD
	distroPath  string
	configPath  string
	workDir     string
	binPath     string
}

func (k *KubeconfigGetter) SetProperties(props []cluster.KubeconfigProperty) {
	for _, prop := range props {
		k.SetProperty(prop.Name, prop.Value)
	}

	k.OperationPhase = &cluster.OperationPhase{}
}

func (k *KubeconfigGetter) SetProperty(name string, value any) {
	switch strings.ToLower(name) {
	case cluster.KubeconfigPropertyFuryctlConf:
		cluster.SetPropertyValue(value, &k.furyctlConf)
	case cluster.KubeconfigPropertyConfigPath:
		cluster.SetPropertyValue(value, &k.configPath)
	case cluster.KubeconfigPropertyWorkDir:
		cluster.SetPropertyValue(value, &k.workDir)
	case cluster.KubeconfigPropertyKfdManifest:
		cluster.SetPropertyValue(value, &k.kfdManifest)
	case cluster.KubeconfigPropertyDistroPath:
		cluster.SetPropertyValue(value, &k.distroPath)
	case cluster.KubeconfigPropertyBinPath:
		cluster.SetPropertyValue(value, &k.binPath)
	default:
		logrus.Debugf("ignoring unknown property %q", name)
	}
}

func (k *KubeconfigGetter) Get() error {
	logrus.Info("Getting kubeconfig...")

	kubeconfigPath := path.Join(k.workDir, "kubeconfig")

	tmpDir, err := os.MkdirTemp("", "fury-kubeconfig-*")
	if err != nil {
		return fmt.Errorf("error creating temporary directory: %w", err)
	}

	defer os.RemoveAll(tmpDir)

	ansibleRunner := ansible.NewRunner(
		execx.NewStdExecutor(),
		ansible.PathsForVersion(k.binPath, k.kfdManifest.Tools.OnPremises.Ansible.Version, tmpDir),
	)

	furyctlMerger, err := k.CreateFuryctlMerger(
		k.distroPath,
		k.configPath,
		"kfd-v1alpha2",
		"onpremises",
	)
	if err != nil {
		return fmt.Errorf("error creating furyctl merger: %w", err)
	}

	mCfg, err := templatex.NewConfigWithoutData(furyctlMerger, []string{})
	if err != nil {
		return fmt.Errorf("error creating template config: %w", err)
	}

	mCfg.Data["kubernetes"] = map[any]any{
		"version": k.kfdManifest.Kubernetes.OnPremises.Version,
	}

	if err := k.CopyFromTemplate(
		mCfg,
		"preflight",
		path.Join(k.distroPath, "templates", cluster.OperationPhasePreFlight, "onpremises"),
		tmpDir,
		k.configPath,
	); err != nil {
		return fmt.Errorf("error copying from template: %w", err)
	}

	if _, err := ansibleRunner.Exec("all", "-m", "ping"); err != nil {
		return fmt.Errorf("error checking hosts: %w", err)
	}

	if _, err := ansibleRunner.Playbook("verify-playbook.yaml"); err != nil {
		return fmt.Errorf("error getting kubeconfig: %w", err)
	}

	kubeconfig, err := os.ReadFile(path.Join(tmpDir, "admin.conf"))
	if err != nil {
		return fmt.Errorf("error reading kubeconfig file: %w", err)
	}

	if err := os.WriteFile(kubeconfigPath, kubeconfig, iox.FullRWPermAccess); err != nil {
		return fmt.Errorf("error writing kubeconfig file: %w", err)
	}

	return nil
}
