// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package distribution

import "github.com/sighupio/furyctl/internal/semver"

type Kind string

func (k Kind) String() string {
	return string(k)
}

func (k Kind) Equals(kk Kind) bool {
	return k == kk
}

type FuryctlConfig struct {
	ApiVersion string `yaml:"apiVersion"`
	Kind       Kind   `yaml:"kind"`
	Spec       struct {
		DistributionVersion semver.Version `yaml:"distributionVersion"`
	} `yaml:"spec"`
}

func (c *FuryctlConfig) UnmarshalYAML(unmarshal func(any) error) error {
	type rawFuryctlConfig FuryctlConfig
	raw := rawFuryctlConfig{}
	if err := unmarshal(&raw); err != nil {
		return err
	}
	*c = FuryctlConfig(raw)

	distroVersion, err := semver.NewVersion(string(c.Spec.DistributionVersion))
	if err != nil {
		return err
	}

	c.Spec.DistributionVersion = distroVersion

	return nil
}

type Manifest struct {
	Version semver.Version `yaml:"version"`
	Modules struct {
		Auth       string `yaml:"auth"`
		Dr         string `yaml:"dr"`
		Ingress    string `yaml:"ingress"`
		Logging    string `yaml:"logging"`
		Monitoring string `yaml:"monitoring"`
		Opa        string `yaml:"opa"`
	} `yaml:"modules"`
	Kubernetes struct {
		Eks struct {
			Version   string `yaml:"version"`
			Installer string `yaml:"installer"`
		} `yaml:"eks"`
	} `yaml:"kubernets"`
	FuryctlSchemas struct {
		Eks []struct {
			ApiVersion string `yaml:"apiVersion"`
			Kind       string `yaml:"kind"`
		} `yaml:"eks"`
	} `yaml:"furyctlSchemas"`
	Tools struct {
		Ansible   string `yaml:"ansible"`
		Furyagent string `yaml:"furyagent"`
		Kubectl   string `yaml:"kubectl"`
		Kustomize string `yaml:"kustomize"`
		Terraform string `yaml:"terraform"`
	} `yaml:"tools"`
}

func (m *Manifest) UnmarshalYAML(unmarshal func(any) error) error {
	type rawKfdManifest Manifest
	raw := rawKfdManifest{}
	if err := unmarshal(&raw); err != nil {
		return err
	}
	*m = Manifest(raw)

	version, err := semver.NewVersion(m.Version.String())
	if err != nil {
		return err
	}

	m.Version = version

	return nil
}
