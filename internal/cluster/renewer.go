// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cluster

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sighupio/furyctl/internal/apis/config"
	"github.com/sighupio/furyctl/internal/tool/ansible"
	kubex "github.com/sighupio/furyctl/internal/x/kube"
	templatex "github.com/sighupio/furyctl/pkg/template"
	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

const (
	// AdminKubeconfigUser is the name that selects the cluster's admin.conf kubeconfig.
	AdminKubeconfigUser = "admin"

	RenewerPropertyFuryctlConf = "furyctlconf"
	RenewerPropertyConfigPath  = "configpath"
	RenewerPropertyKfdManifest = "kfdmanifest"
	RenewerPropertyDistroPath  = "distropath"
	RenewerPropertyBinPath     = "binpath"
	RenewerPropertyWorkDir     = "workdir"
)

var (
	ErrRenewNotSupported         = errors.New("renew is not supported")
	ErrUnsupportedByDistribution = errors.New("the distribution does not have this playbook")

	renewerFactories = make(map[string]map[string]RenewerFactory) //nolint:gochecknoglobals, lll // This patterns requires renewerFactories as global to work with init function.
)

type RenewerFactory func(configPath string, props []RenewerProperty) (Renewer, error) //nolint:lll // This pattern requires RenewerFactory as global to work with init function.

type RenewerProperty struct {
	Name  string
	Value any
}

type Renewer interface {
	SetProperties(props []RenewerProperty)
	SetProperty(name string, value any)
	// Users returns the kubeconfig names the cluster's configuration defines, "admin" included.
	Users() []string
	RenewCertificates() error
	// RenewKubeconfigs renews the kubeconfig files of the given users and writes them to the
	// working directory.
	RenewKubeconfigs(users []string) error
}

func NewRenewer(
	minimalConf config.Furyctl,
	kfdManifest config.KFD,
	distroPath string,
	configPath string,
	binPath string,
) (Renewer, error) {
	lcAPIVersion := strings.ToLower(minimalConf.APIVersion)
	lcResourceType := strings.ToLower(minimalConf.Kind)

	if factoryFn, ok := renewerFactories[lcAPIVersion][lcResourceType]; ok {
		return factoryFn(configPath, []RenewerProperty{
			{
				Name:  RenewerPropertyKfdManifest,
				Value: kfdManifest,
			},
			{
				Name:  RenewerPropertyDistroPath,
				Value: distroPath,
			},
			{
				Name:  RenewerPropertyBinPath,
				Value: binPath,
			},
		})
	}

	return nil, fmt.Errorf("%w -  type '%s' api version '%s'", errResourceNotSupported, lcResourceType, lcAPIVersion)
}

func RegisterRenewerFactory(apiVersion, kind string, factory RenewerFactory) {
	lcAPIVersion := strings.ToLower(apiVersion)
	lcKind := strings.ToLower(kind)

	if _, ok := renewerFactories[lcAPIVersion]; !ok {
		renewerFactories[lcAPIVersion] = make(map[string]RenewerFactory)
	}

	renewerFactories[lcAPIVersion][lcKind] = factory
}

func NewRenewerFactory[T Renewer, S any](cc T) RenewerFactory {
	return func(configPath string, props []RenewerProperty) (Renewer, error) {
		furyctlConf, err := yamlx.FromFileV3[S](configPath)
		if err != nil {
			return nil, err
		}

		cc.SetProperty(RenewerPropertyConfigPath, configPath)
		cc.SetProperty(RenewerPropertyFuryctlConf, furyctlConf)
		cc.SetProperties(props)

		return cc, nil
	}
}

// RunPlaybook runs the named playbook from workDir. A playbook that the rendered distribution does
// not have means that the SD version is older than the operation. The ansible error does not say
// that, thus this function looks for the file first.
func RunPlaybook(runner *ansible.Runner, workDir, sdVersion, name string, args ...string) error {
	if _, err := os.Stat(filepath.Join(workDir, name)); err != nil {
		return fmt.Errorf(
			"%w: SD %s does not have %q. Upgrade the distribution to a version that has it",
			ErrUnsupportedByDistribution, sdVersion, name,
		)
	}

	if _, err := runner.Playbook(append([]string{name}, args...)...); err != nil {
		return fmt.Errorf("error running the %q playbook: %w", name, err)
	}

	return nil
}

// SetEmptyRenderDefaults fills the template keys the kubernetes phase templates read but the renew
// path does not use. They must be present: the templates render with `missingkey=error`.
func SetEmptyRenderDefaults(cfg *templatex.Config) {
	cfg.Data["paths"] = map[any]any{
		"helm":       "",
		"helmfile":   "",
		"kubectl":    "",
		"kustomize":  "",
		"terraform":  "",
		"vendorPath": "",
		"yq":         "",
	}

	cfg.Data["options"] = map[any]any{
		"skipPodsRunningCheck": false,
		"podRunningTimeout":    "",
	}
}

// KubeconfigsExtraVars builds the `-e` argument that tells the renewal playbook which kubeconfig
// files to renew. It must stay JSON: ansible splits the `key=value` form on whitespace, and the
// schema does not forbid a space in a user name.
func KubeconfigsExtraVars(users []string) (string, error) {
	out, err := json.Marshal(map[string][]string{"renew_kubeconfigs_users": users})
	if err != nil {
		return "", fmt.Errorf("error building the ansible extra variables: %w", err)
	}

	return string(out), nil
}

// CopyRenewedKubeconfigs copies the kubeconfig files the playbook downloaded into srcDir to the
// working directory. The admin kubeconfig keeps the `kubeconfig` name that `furyctl apply` writes.
func CopyRenewedKubeconfigs(srcDir string, users []string) error {
	for _, user := range users {
		name := user + ".kubeconfig"
		dst := name

		if user == AdminKubeconfigUser {
			name, dst = "admin.conf", "kubeconfig"
		}

		if err := kubex.CopyToWorkDir(filepath.Join(srcDir, name), dst); err != nil {
			return fmt.Errorf("error copying the %s kubeconfig file: %w", user, err)
		}
	}

	return nil
}

// UnsupportedRenewer is the Renewer of the kinds that furyctl cannot renew.
type UnsupportedRenewer struct {
	Kind string
}

func (*UnsupportedRenewer) SetProperties(_ []RenewerProperty) {}

func (*UnsupportedRenewer) SetProperty(_ string, _ any) {}

func (*UnsupportedRenewer) Users() []string { return nil }

func (u *UnsupportedRenewer) RenewCertificates() error { return u.err() }

func (u *UnsupportedRenewer) RenewKubeconfigs(_ []string) error { return u.err() }

func (u *UnsupportedRenewer) err() error {
	return fmt.Errorf("%w for the %s kind", ErrRenewNotSupported, u.Kind)
}
