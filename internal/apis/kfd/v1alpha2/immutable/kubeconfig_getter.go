// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package immutable

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/furyctl/internal/apis/config"
	preflightx "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/immutable/preflight"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/immutable/public"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/tool/ansible"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
	templatex "github.com/sighupio/furyctl/pkg/template"
)

var errNoKubeconfig = errors.New("no control plane host gave its kubeconfig")

type KubeconfigGetter struct {
	*cluster.OperationPhase

	furyctlConf public.ImmutableKfdV1Alpha2
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
		ansible.PathsForVersion(k.binPath, k.kfdManifest.Tools.Immutable.Ansible.Version, tmpDir),
	)

	furyctlMerger, err := k.CreateFuryctlMerger(
		k.distroPath,
		k.configPath,
		"kfd-v1alpha2",
		"immutable",
	)
	if err != nil {
		return fmt.Errorf("error creating furyctl merger: %w", err)
	}

	mCfg, err := templatex.NewConfigWithoutData(furyctlMerger, []string{})
	if err != nil {
		return fmt.Errorf("error creating template config: %w", err)
	}

	mCfg.Data["kubernetes"] = map[any]any{
		"version": k.kfdManifest.Kubernetes.Immutable.Version,
	}

	if err := k.CopyFromTemplate(
		mCfg,
		"preflight",
		path.Join(k.distroPath, "templates", cluster.OperationPhasePreFlight, "immutable"),
		tmpDir,
		k.configPath,
	); err != nil {
		return fmt.Errorf("error copying from template: %w", err)
	}

	adminConfPlaybook, err := preflightx.AdminConfPlaybookName(tmpDir)
	if err != nil {
		return fmt.Errorf("error selecting admin.conf playbook: %w", err)
	}

	// The playbook of a current distribution does not fail when a host does not answer, and it
	// fetches admin.conf from a control plane host that holds one. One control plane host that
	// answers is therefore enough. The playbook of an older distribution fails when a host does
	// not answer, and the command then stops here with that error.
	if _, err := ansibleRunner.Playbook(adminConfPlaybook); err != nil {
		return fmt.Errorf("error getting kubeconfig: %w", err)
	}

	kubeconfig, err := os.ReadFile(path.Join(tmpDir, "admin.conf"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf(
				"%w: make sure that at least one control plane host answers and holds "+
					"/etc/kubernetes/admin.conf",
				errNoKubeconfig,
			)
		}

		return fmt.Errorf("error reading kubeconfig file: %w", err)
	}

	if err := os.WriteFile(kubeconfigPath, kubeconfig, iox.FullRWPermAccess); err != nil {
		return fmt.Errorf("error writing kubeconfig file: %w", err)
	}

	return nil
}
