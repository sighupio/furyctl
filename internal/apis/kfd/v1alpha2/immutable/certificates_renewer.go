// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package immutable

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/fury-distribution/pkg/apis/immutable/v1alpha2/public"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/tool/ansible"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	"github.com/sighupio/furyctl/pkg/template"
)

type CertificatesRenewer struct {
	*cluster.OperationPhase
	furyctlConf public.ImmutableKfdV1Alpha2
	kfdManifest config.KFD
	distroPath  string
	configPath  string
}

func (c *CertificatesRenewer) SetProperties(props []cluster.CertificatesRenewerProperty) {
	for _, prop := range props {
		c.SetProperty(prop.Name, prop.Value)
	}

	c.OperationPhase = &cluster.OperationPhase{}
}

func (c *CertificatesRenewer) SetProperty(name string, value any) {
	switch strings.ToLower(name) {
	case cluster.CertificatesRenewerPropertyFuryctlConf:
		if s, ok := value.(public.ImmutableKfdV1Alpha2); ok {
			c.furyctlConf = s
		}

	case cluster.CertificatesRenewerPropertyConfigPath:
		if s, ok := value.(string); ok {
			c.configPath = s
		}

	case cluster.CertificatesRenewerPropertyKfdManifest:
		if s, ok := value.(config.KFD); ok {
			c.kfdManifest = s
		}

	case cluster.CertificatesRenewerPropertyDistroPath:
		if s, ok := value.(string); ok {
			c.distroPath = s
		}
	}
}

func (c *CertificatesRenewer) Renew() error {
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

	furyctlMerger, err := c.CreateFuryctlMerger(
		c.distroPath,
		c.configPath,
		"kfd-v1alpha2",
		"immutable",
	)
	if err != nil {
		return fmt.Errorf("error creating furyctl merger: %w", err)
	}

	mCfg, err := template.NewConfigWithoutData(furyctlMerger, []string{})
	if err != nil {
		return fmt.Errorf("error creating template config: %w", err)
	}

	mCfg.Data["kubernetes"] = map[any]any{
		"version": c.kfdManifest.Kubernetes.OnPremises.Version,
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

	if err := c.CopyFromTemplate(
		mCfg,
		"kubernetes",
		path.Join(c.distroPath, "templates", cluster.OperationPhaseKubernetes, "immutable"),
		tmpDir,
		c.configPath,
	); err != nil {
		return fmt.Errorf("error copying from template: %w", err)
	}

	if _, err := ansibleRunner.Exec("all", "-m", "ping"); err != nil {
		return fmt.Errorf("error checking hosts: %w", err)
	}

	if _, err := ansibleRunner.Playbook("98.cluster-certificates-renewal.yaml"); err != nil {
		return fmt.Errorf("error renewing certificates: %w", err)
	}

	return nil
}
