// Copyright © 2018 Sighup SRL support@sighup.io
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"fmt"
	"log"
	"path"
	"strings"
)

const (
	configFile              = "Furyfile"
	httpsRepoPrefix         = "git::https://github.com/sighupio/fury-kubernetes"
	sshRepoPrefix           = "git@github.com:sighupio/fury-kubernetes"
	defaultVendorFolderName = "vendor"
)

// Furyconf is reponsible for the structure of the Furyfile
type Furyconf struct {
	VendorFolderName string          `yaml:"vendorFolderName"`
	Versions         VersionPattern  `yaml:"versions"`
	Roles            []Package       `yaml:"roles"`
	Modules          []Package       `yaml:"modules"`
	Bases            []Package       `yaml:"bases"`
	Provider         ProviderPattern `mapstructure:"provider"`
}

// ProviderPattern is the abstraction of the following structure:
//provider:
//   modules:
//     aws
//      - uri: https://github.com/terraform-aws-modules
//        label: ufficial-modules
type ProviderPattern map[string]ProviderKind

// ProviderKind is the abstraction of the following structure:
//
// modules:
//   aws
//    - uri: https://github.com/terraform-aws-modules
//      label: ufficial-modules
type ProviderKind map[string][]RegistrySpec

//RegistrySpec contains the couple uri/label to identify each tf new repo declared
type RegistrySpec struct {
	BaseURI string `mapstructure:"url"`
	Label   string `mapstructure:"label"`
}

//VersionPattern Map from glob pattern to version associated (e.g. {"aws/*" : "v1.15.4-1"}
type VersionPattern map[string]string

// Package is the type to contain the definition of a single package
type Package struct {
	Name        string `yaml:"name"`
	Version     string `yaml:"version"`
	url         string
	dir         string
	kind        string
	ProviderOpt ProviderOptSpec `mapstructure:"provider"`
	Registry    bool            `mapstructure:"registry"`
}

// ProviderSpec is the type that allows to explicit name of cloud provider and referenced label
type ProviderOptSpec struct {
	Name  string `mapstructure:"name"`
	Label string `mapstructure:"label"`
}

// Validate is used for validation of configuration and initization of default parameters
func (f *Furyconf) Validate() error {
	if f.VendorFolderName == "" {
		f.VendorFolderName = defaultVendorFolderName
	}
	return nil
}

// Parse reads the furyconf structs and created a list of packaged to be downloaded
func (f *Furyconf) Parse() ([]Package, error) {
	pkgs := make([]Package, 0, 0)
	// First we aggreggate all packages in one single list
	for _, v := range f.Roles {
		v.kind = "roles"
		pkgs = append(pkgs, v)
	}
	for _, v := range f.Modules {
		v.kind = "modules"
		pkgs = append(pkgs, v)
	}
	for _, v := range f.Bases {
		v.kind = "katalog"
		pkgs = append(pkgs, v)
	}
	repoPrefix := sshRepoPrefix
	dotGitParticle := ""

	if https {
		repoPrefix = httpsRepoPrefix
		dotGitParticle = ".git"
	}

	// Now we generate the download url and local dir
	for i := 0; i < len(pkgs); i++ {
		version := pkgs[i].Version

		if version == "" {
			for k, v := range f.Versions {
				if strings.HasPrefix(pkgs[i].Name, k) {
					version = v
					log.Printf("using %v for package %s\n", version, pkgs[i].Name)
					break
				}
			}
		}
		registry := pkgs[i].Registry
		cloudPlatform := pkgs[i].ProviderOpt
		pkgKind := pkgs[i].kind

		pkgs[i].url = newURLSpec(repoPrefix, strings.Split(pkgs[i].Name, "/"), dotGitParticle, pkgKind, version, registry, cloudPlatform,  newKind(pkgKind, f.Provider)).getConsumableUrl()

		pkgs[i].dir = newDir(f.VendorFolderName, pkgKind, pkgs[i].Name, registry, cloudPlatform).getConsumableDirectory()

	}

	return pkgs, nil
}

func newKind(kind string, provider ProviderPattern) ProviderKind {
	return provider[kind]
}

func (k *ProviderKind) getLabeledURI(providerName, label string) (string, error) {
	for name, providerSpecList := range *k {

		if name != providerName {
			continue
		}
		for _, providerMap := range providerSpecList {

			if providerMap.Label != label {
				continue
			}

			return fmt.Sprintf("git::%s", providerMap.BaseURI), nil

		}

	}
	return "", fmt.Errorf("no label %s found", label)
}

func (k *ProviderKind) pickCloudProviderURL(cloudProvider ProviderOptSpec) string {

	url, err := k.getLabeledURI(cloudProvider.Name, cloudProvider.Label)

	if err != nil {
		log.Fatal(err)
	}

	return url
}

// DirSpec is the abstraction of the fields needed for generating a destination directory
type DirSpec struct {
	VendorFolder string
	Kind         string
	Name         string
	Registry     bool
	Provider     ProviderOptSpec
}

func newDir(folder, kind, name string, registry bool, provider ProviderOptSpec) *DirSpec {
	return &DirSpec{
		VendorFolder: folder,
		Kind:         kind,
		Name:         name,
		Registry:     registry,
		Provider:     provider,
	}
}
// getConsumableDirectory returns a directory we can write to
func (d *DirSpec) getConsumableDirectory() string {
	if d.Registry {
		return fmt.Sprintf("%s/%s/%s/%s/%s", d.VendorFolder, d.Kind, d.Provider.Label, d.Provider.Name, d.Name)
	}
	return fmt.Sprintf("%s/%s/%s", d.VendorFolder, d.Kind, d.Name)
}

//URLSpec is the representation of the fields needed to elaborate a url
type URLSpec struct {
	Prefix         string
	Blocks         []string
	DotGitParticle string
	Kind           string
	Version        string
	Registry       bool
	CloudProvider  ProviderOptSpec
	KindSpec       ProviderKind
}

// newUrl initialize the URLSpec struct
func newURLSpec(prefix string, blocks []string, dotGitParticle, kind, version string, registry bool, cloud ProviderOptSpec, kindSpec ProviderKind) *URLSpec {
	return &URLSpec{
		Registry:       registry,
		Prefix:         prefix,
		Blocks:         blocks,
		DotGitParticle: dotGitParticle,
		Kind:           kind,
		Version:        version,
		CloudProvider:  cloud,
		KindSpec:       kindSpec,
	}
}
//getConsumableUrl returns an url that can be used for download
func (n *URLSpec) getConsumableUrl() string {

	if !n.Registry {
		return n.getURLFromCompanyRepos()
	}

	return fmt.Sprintf("%s/%s%s?ref=%s", n.KindSpec.pickCloudProviderURL(n.CloudProvider), n.Blocks[0], ".git", n.Version)

}

func (n *URLSpec) getURLFromCompanyRepos() string {

	if len(n.Blocks) == 0 {
		// todo should return error?
		return ""
	}

	if len(n.Blocks) == 1 {
		return fmt.Sprintf("%s-%s%s//%s?ref=%s", n.Prefix, n.Blocks[0], n.DotGitParticle, n.Kind, n.Version)
	}
	// always len(n.Blocks) >= 2 {
	var remainingBlocks string

	for i := 1; i < len(n.Blocks); i++ {
		remainingBlocks = path.Join(remainingBlocks, n.Blocks[i])
	}

	return fmt.Sprintf("%s-%s%s//%s/%s?ref=%s", n.Prefix, n.Blocks[0], n.DotGitParticle, n.Kind, remainingBlocks, n.Version)

}
