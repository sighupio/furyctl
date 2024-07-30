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

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/fury-distribution/pkg/apis/onpremises/v1alpha2/public"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/tool/ansible"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	"github.com/sighupio/furyctl/pkg/template"
)

type CertificatesRenewer struct {
	*cluster.OperationPhase
	furyctlConf public.OnpremisesKfdV1Alpha2
	kfdManifest config.KFD
	distroPath  string
	configPath  string
	outDir      string
}

func (k *CertificatesRenewer) SetProperties(props []cluster.CertificatesRenewerProperty) {
	for _, prop := range props {
		k.SetProperty(prop.Name, prop.Value)
	}

	k.OperationPhase = &cluster.OperationPhase{}
}

func (k *CertificatesRenewer) SetProperty(name string, value any) {
	lcName := strings.ToLower(name)

	switch lcName {
	case cluster.KubeconfigPropertyFuryctlConf:
		if s, ok := value.(public.OnpremisesKfdV1Alpha2); ok {
			k.furyctlConf = s
		}

	case cluster.KubeconfigPropertyConfigPath:
		if s, ok := value.(string); ok {
			k.configPath = s
		}

	case cluster.KubeconfigPropertyOutdir:
		if s, ok := value.(string); ok {
			k.outDir = s
		}

	case cluster.KubeconfigPropertyKfdManifest:
		if s, ok := value.(config.KFD); ok {
			k.kfdManifest = s
		}

	case cluster.KubeconfigPropertyDistroPath:
		if s, ok := value.(string); ok {
			k.distroPath = s
		}
	}
}

func (k *CertificatesRenewer) Renew() error {
	logrus.Info("Renewing certificates...")

	tmpDir, err := os.MkdirTemp("", "fury-certificates-renewer-*")
	if err != nil {
		return fmt.Errorf("error creating temporary directory: %w", err)
	}

	defer os.RemoveAll(tmpDir)

	ansibleRunner := ansible.NewRunner(
		execx.NewStdExecutor(),
		ansible.Paths{
			Ansible:         "ansible",
			AnsiblePlaybook: "ansible-playbook",
			WorkDir:         tmpDir,
		},
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

	mCfg, err := template.NewConfigWithoutData(furyctlMerger, []string{})
	if err != nil {
		return fmt.Errorf("error creating template config: %w", err)
	}

	mCfg.Data["kubernetes"] = map[any]any{
		"version": k.kfdManifest.Kubernetes.OnPremises.Version,
	}

	mCfg.Data["paths"] = map[any]any{
		"helm":       "",
		"helmfile":   "",
		"kubectl":    "",
		"kustomize":  "",
		"terraform":  "",
		"vendorPath": "",
		"yq":         "",
	}

	mCfg.Data["options"] = map[any]any{
		"skipPodsRunningCheck": false,
		"podRunningTimeout":    "",
	}

	if err := k.CopyFromTemplate(
		mCfg,
		"kubernetes",
		path.Join(k.distroPath, "templates", cluster.OperationPhaseKubernetes, "onpremises"),
		tmpDir,
		k.configPath,
	); err != nil {
		return fmt.Errorf("error copying from template: %w", err)
	}

	if _, err := ansibleRunner.Exec("all", "-m", "ping"); err != nil {
		return fmt.Errorf("error checking hosts: %w", err)
	}

	if _, err := ansibleRunner.Playbook("renew-certificates.yaml"); err != nil {
		return fmt.Errorf("error renewing certificates: %w", err)
	}

	return nil
}
