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
	VendorFolderName string            `yaml:"vendorFolderName"`
	Versions         VersionPattern    `yaml:"versions"`
	Roles            []Package         `yaml:"roles"`
	Modules          []Package         `yaml:"modules"`
	Bases            []Package         `yaml:"bases"`
	TfRegistries     TfRegistryPattern `mapstructure:"tfRepos"`
}

// TfRegistryPattern is the abstraction of the following structure:
//
// aws:
//   - uri: https://github.com/terraform-aws-modules
//     label: ufficial-modules
type TfRegistryPattern map[string][]TfRegistry

//TfRegistry contains the couple uri/label to identify each tf new repo declared
type TfRegistry struct {
	BaseURI string `mapstructure:"url"`
	Label   string `yaml:"label"`
}

//VersionPattern Map from glob pattern to version associated (e.g. {"aws/*" : "v1.15.4-1"}
type VersionPattern map[string]string

// Package is the type to contain the definition of a single package
type Package struct {
	Name     string `yaml:"name"`
	Version  string `yaml:"version"`
	url      string
	dir      string
	kind     string
	Provider ProviderSpec `yaml:"provider"`
	Registry bool         `yaml:"registry"`
}

// ProviderSpec is the type that allows to explicit name of cloud provider and referenced label
type ProviderSpec struct {
	Name  string `yaml:"name"`
	Label string `yaml:"label"`
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

	// Now we generate the dowload url and local dir
	for i := 0; i < len(pkgs); i++ {
		url := new(URLSpec)
		directory := new(DirSpec)
		var urlPrefix string
		version := pkgs[i].Version
		registry := pkgs[i].Registry
		cloud := pkgs[i].Provider
		urlPrefix = repoPrefix
		if registry {
			urlPrefix = f.pickCloudProviderURL(cloud)
			dotGitParticle = ".git"
		}
		if version == "" {
			for k, v := range f.Versions {
				if strings.HasPrefix(pkgs[i].Name, k) {
					version = v
					log.Printf("using %v for package %s\n", version, pkgs[i].Name)
					break
				}
			}
		}
		block := strings.Split(pkgs[i].Name, "/")
		url = newURL(urlPrefix, block, dotGitParticle, pkgs[i].kind, version, registry)
		pkgs[i].url = url.strategy()
		directory = newDir(f.VendorFolderName, pkgs[i].kind, pkgs[i].Name, pkgs[i].Registry, pkgs[i].Provider)
		pkgs[i].dir = directory.strategy()
	}

	return pkgs, nil
}

func (f *Furyconf) tfURI(providerMap TfRegistry, label string) (string, error) {
	if providerMap.Label == label {
		return fmt.Sprintf("git::%s", providerMap.BaseURI), nil
	}
	return "", fmt.Errorf("the label %s is not present\n", label)
}

func (f *Furyconf) getTfURI(providerName, label string) string {
	for name, providerSpecList := range f.TfRegistries {
		if name == providerName {
			for _, providerMap := range providerSpecList {
				uri, err := f.tfURI(providerMap, label)
				if err != nil {
					log.Fatal(err)
				}
				return uri
			}
		}
	}
	return ""
}

func (f *Furyconf) pickCloudProviderURL(cloudProvider ProviderSpec) string {
	name := cloudProvider.Name
	label := cloudProvider.Label
	return f.getTfURI(name, label)
}

type DirSpec struct {
	VendorFolder string
	Kind         string
	Name         string
	Registry     bool
	Provider     ProviderSpec
}

func newDir(folder, kind, name string, registry bool, provider ProviderSpec) *DirSpec {
	return &DirSpec{
		VendorFolder: folder,
		Kind:         kind,
		Name:         name,
		Registry:     registry,
		Provider:     provider,
	}
}

func (d *DirSpec) strategy() string {
	if d.Registry {
		return fmt.Sprintf("%s/%s/%s/%s/%s", d.VendorFolder, d.Kind, d.Provider.Label, d.Provider.Name, d.Name)
	}
	return fmt.Sprintf("%s/%s/%s", d.VendorFolder, d.Kind, d.Name)
}

//URLSpec is the rappresentation of the fields needed to elaborate a url
type URLSpec struct {
	Prefix         string
	Blocks         []string
	DotGitParticle string
	Kind           string
	Version        string
	Registry       bool
}

// newUrl initialize the URLSpec struct
func newURL(prefix string, blocks []string, dotGitParticle, kind, version string, registry bool) *URLSpec {
	return &URLSpec{
		Registry:       registry,
		Prefix:         prefix,
		Blocks:         blocks,
		DotGitParticle: dotGitParticle,
		Kind:           kind,
		Version:        version,
	}
}

func (n *URLSpec) strategy() string {
	var url string
	if n.Registry {
		url = fmt.Sprintf("%s/%s%s?ref=%s", n.Prefix, n.Blocks[0], n.DotGitParticle, n.Version)
	} else {
		url = n.internalBehaviourURL()
	}
	return url
}

func (n *URLSpec) internalBehaviourURL() string {
	var url string
	if len(n.Blocks) == 2 {
		url = fmt.Sprintf("%s-%s%s//%s/%s?ref=%s", n.Prefix, n.Blocks[0], n.DotGitParticle, n.Kind, n.Blocks[1], n.Version)
	} else if len(n.Blocks) == 1 {
		url = fmt.Sprintf("%s-%s%s//%s?ref=%s", n.Prefix, n.Blocks[0], n.DotGitParticle, n.Kind, n.Version)
	}
	return url
}
