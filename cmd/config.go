// Copyright (c) 2022 SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
)

const (
	configFile              = "Furyfile"
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
//        label: official-modules
type ProviderPattern map[string]ProviderKind

// ProviderKind is the abstraction of the following structure:
//
// modules:
//   aws
//    - uri: https://github.com/terraform-aws-modules
//      label: official-modules
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
	Name         string          `yaml:"name"`
	Version      string          `yaml:"version"`
	Url          string          `yaml:"url"`
	Dir          string          `yaml:"dir"`
	Kind         string          `yaml:"kind"`
	ProviderOpt  ProviderOptSpec `mapstructure:"provider"`
	ProviderKind ProviderKind    `mapstructure:"providerKind"`
	Registry     bool            `mapstructure:"registry"`
}

// ProviderOptSpec is the type that allows to explicit name of cloud provider and referenced label
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
func (f *Furyconf) Parse(prefix string) ([]Package, error) {
	pkgs := make([]Package, 0, 0)
	// First we aggregate all packages in one single list
	for _, v := range f.Roles {
		v.Kind = "roles"
		if strings.HasPrefix(v.Name, prefix) {
			pkgs = append(pkgs, v)
		}
	}
	for _, v := range f.Modules {
		v.Kind = "modules"
		if strings.HasPrefix(v.Name, prefix) {
			pkgs = append(pkgs, v)
		}
	}
	for _, v := range f.Bases {
		v.Kind = "katalog"
		if strings.HasPrefix(v.Name, prefix) {
			pkgs = append(pkgs, v)
		}
	}

	// Now we parse the local dir name
	for i, _ := range pkgs {
		if pkgs[i].Version == "" {
			for k, v := range f.Versions {
				if strings.HasPrefix(pkgs[i].Name, k) {
					pkgs[i].Version = v
					logrus.Infof("using %v for package %s", version, pkgs[i].Name)
					break
				}
			}
		}

		pkgs[i].ProviderKind = f.Provider[pkgs[i].Kind]
		pkgs[i].Dir = newDir(f.VendorFolderName, pkgs[i]).getConsumableDirectory()
	}

	return pkgs, nil
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
		logrus.Fatal(err)
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

func newDir(vendorFolder string, pkg Package) *DirSpec {
	return &DirSpec{
		VendorFolder: vendorFolder,
		Kind:         pkg.Kind,
		Name:         pkg.Name,
		Registry:     pkg.Registry,
		Provider:     pkg.ProviderOpt,
	}
}

// getConsumableDirectory returns a directory we can write to
func (d *DirSpec) getConsumableDirectory() string {
	if d.Registry {
		return fmt.Sprintf("%s/%s/%s/%s/%s", d.VendorFolder, d.Kind, d.Provider.Label, d.Provider.Name, d.Name)
	}
	return fmt.Sprintf("%s/%s/%s", d.VendorFolder, d.Kind, d.Name)
}
