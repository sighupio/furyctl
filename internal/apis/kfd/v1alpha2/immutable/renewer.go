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

	"github.com/sighupio/furyctl/internal/apis/config"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/immutable/create"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/immutable/public"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/tool/ansible"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	templatex "github.com/sighupio/furyctl/pkg/template"
)

type Renewer struct {
	*cluster.OperationPhase

	furyctlConf public.ImmutableKfdV1Alpha2
	kfdManifest config.KFD
	distroPath  string
	configPath  string
	binPath     string
	workDir     string
}

func (c *Renewer) SetProperties(props []cluster.RenewerProperty) {
	for _, prop := range props {
		c.SetProperty(prop.Name, prop.Value)
	}

	c.OperationPhase = &cluster.OperationPhase{}
}

func (c *Renewer) SetProperty(name string, value any) {
	switch strings.ToLower(name) {
	case cluster.RenewerPropertyFuryctlConf:
		cluster.SetPropertyValue(value, &c.furyctlConf)
	case cluster.RenewerPropertyConfigPath:
		cluster.SetPropertyValue(value, &c.configPath)
	case cluster.RenewerPropertyKfdManifest:
		cluster.SetPropertyValue(value, &c.kfdManifest)
	case cluster.RenewerPropertyDistroPath:
		cluster.SetPropertyValue(value, &c.distroPath)
	case cluster.RenewerPropertyBinPath:
		cluster.SetPropertyValue(value, &c.binPath)
	case cluster.RenewerPropertyWorkDir:
		cluster.SetPropertyValue(value, &c.workDir)
	default:
		logrus.Debugf("ignoring unknown property %q", name)
	}
}

func (c *Renewer) Users() []string {
	names := []string{cluster.AdminKubeconfigUser}

	if c.furyctlConf.Spec.Kubernetes.Advanced != nil && c.furyctlConf.Spec.Kubernetes.Advanced.Users != nil {
		names = append(names, c.furyctlConf.Spec.Kubernetes.Advanced.Users.Names...)
	}

	return names
}

func (c *Renewer) RenewCertificates() error {
	logrus.Info("Renewing certificates...")

	tmpDir, err := os.MkdirTemp("", "fury-renewer-*")
	if err != nil {
		return fmt.Errorf("error creating temporary directory: %w", err)
	}

	defer os.RemoveAll(tmpDir)

	runner, err := c.render(tmpDir)
	if err != nil {
		return err
	}

	if err := cluster.RunPlaybook(runner, tmpDir, c.kfdManifest.Version, "renew-certificates.yaml"); err != nil {
		return fmt.Errorf("error renewing certificates: %w", err)
	}

	return nil
}

func (c *Renewer) RenewKubeconfigs(users []string) error {
	logrus.Info("Renewing kubeconfig files...")

	tmpDir, err := os.MkdirTemp("", "fury-renewer-*")
	if err != nil {
		return fmt.Errorf("error creating temporary directory: %w", err)
	}

	defer os.RemoveAll(tmpDir)

	runner, err := c.render(tmpDir)
	if err != nil {
		return err
	}

	extraVars, err := cluster.KubeconfigsExtraVars(users)
	if err != nil {
		return fmt.Errorf("error renewing the kubeconfig files: %w", err)
	}

	if err := cluster.RunPlaybook(
		runner, tmpDir, c.kfdManifest.Version, "renew-kubeconfigs.yaml", "-e", extraVars,
	); err != nil {
		return fmt.Errorf("error renewing the kubeconfig files: %w", err)
	}

	if err := cluster.CopyRenewedKubeconfigs(tmpDir, users); err != nil {
		return fmt.Errorf("error collecting the renewed kubeconfig files: %w", err)
	}

	return nil
}

// render writes the kubernetes phase templates into renderDir (an ephemeral temp dir) and returns an
// ansible runner rooted there, with the hosts already checked. Vendor/kubectl resolution uses the
// persistent c.workDir, not renderDir.
func (c *Renewer) render(renderDir string) (*ansible.Runner, error) {
	// Root the phase at the cluster's workDir/kubernetes so version vars resolve the vendored
	// immutable.yaml and KubectlPath, like the create phase (hosts.yaml reads .versions.kubectl_bin
	// under missingkey=error).
	c.OperationPhase = cluster.NewOperationPhase(
		path.Join(c.workDir, cluster.OperationPhaseKubernetes),
		c.kfdManifest.Tools,
		c.binPath,
	)

	ansibleRunner := ansible.NewRunner(
		execx.NewStdExecutor(),
		ansible.PathsForVersion(c.binPath, c.kfdManifest.Tools.Immutable.Ansible.Version, renderDir),
	)

	furyctlMerger, err := c.CreateFuryctlMerger(
		c.distroPath,
		c.configPath,
		"kfd-v1alpha2",
		"immutable",
	)
	if err != nil {
		return nil, fmt.Errorf("error creating furyctl merger: %w", err)
	}

	mCfg, err := templatex.NewConfigWithoutData(furyctlMerger, []string{})
	if err != nil {
		return nil, fmt.Errorf("error creating template config: %w", err)
	}

	version := c.kfdManifest.Kubernetes.Immutable.Version

	mCfg.Data["kubernetes"] = map[any]any{
		"version": version,
	}

	// Inject the same "versions" data as the create phase; hosts.yaml fails on the missing key otherwise.
	versionVars, err := create.VersionVarsForPhase(c.Path, version, c.KubectlPath)
	if err != nil {
		return nil, fmt.Errorf("error building version vars: %w", err)
	}

	mCfg.Data["versions"] = versionVars

	cluster.SetEmptyRenderDefaults(&mCfg)

	if err := c.CopyFromTemplate(
		mCfg,
		"kubernetes",
		path.Join(c.distroPath, "templates", cluster.OperationPhaseKubernetes, "immutable"),
		renderDir,
		c.configPath,
	); err != nil {
		return nil, fmt.Errorf("error copying from template: %w", err)
	}

	if _, err := ansibleRunner.Exec("all", "-m", "ping"); err != nil {
		return nil, fmt.Errorf("error checking hosts: %w", err)
	}

	return ansibleRunner, nil
}
