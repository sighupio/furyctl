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
	VendorFolderName string         `yaml:"vendorFolderName"`
	Versions         VersionPattern `yaml:"versions"`
	Roles            []Package      `yaml:"roles"`
	Modules          []Package      `yaml:"modules"`
	Bases            []Package      `yaml:"bases"`
}

// Map from glob pattern to version associated (e.g. {"aws/*" : "v1.15.4-1"}
type VersionPattern map[string]string

// Package is the type to contain the definition of a single package
type Package struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
	url     string
	dir     string
	kind    string
}

// Validate is used for validation of configuration and initization of default paramethers
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
		block := strings.Split(pkgs[i].Name, "/")
		if len(block) == 2 {
			pkgs[i].url = fmt.Sprintf("%s-%s%s//%s/%s?ref=%s", repoPrefix, block[0], dotGitParticle, pkgs[i].kind, block[1], version)
		} else if len(block) == 1 {
			pkgs[i].url = fmt.Sprintf("%s-%s%s//%s?ref=%s", repoPrefix, block[0], dotGitParticle, pkgs[i].kind, version)
		}
		pkgs[i].dir = fmt.Sprintf("%s/%s/%s", f.VendorFolderName, pkgs[i].kind, pkgs[i].Name)
	}
	return pkgs, nil
}
